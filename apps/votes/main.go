package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const recentPollLimit = 50

type pollInput struct {
	Question string   `json:"question"`
	Choices  []string `json:"choices"`
}

type voteInput struct {
	Choice int `json:"choice"`
}

type choice struct {
	Label string `json:"label"`
	Votes int64  `json:"votes"`
}

type poll struct {
	ID         string   `json:"id"`
	Question   string   `json:"question"`
	Choices    []choice `json:"choices"`
	TotalVotes int64    `json:"total_votes"`
	CreatedAt  string   `json:"created_at"`
}

type server struct {
	valkey *redis.Client
}

func main() {
	ctx := context.Background()
	shutdownTelemetry, err := configureTelemetry(ctx, "votes")
	if err != nil {
		slog.Error("configure telemetry", "error", err)
		os.Exit(1)
	}
	defer shutdownTelemetry(context.Background())

	valkey := redis.NewClient(&redis.Options{
		Addr:     env("VALKEY_ADDR", "localhost:6379"),
		Password: os.Getenv("VALKEY_PASSWORD"),
	})
	defer valkey.Close()

	app := &server{valkey: valkey}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /polls", app.createPoll)
	mux.HandleFunc("GET /polls", app.listPolls)
	mux.HandleFunc("GET /polls/{id}", app.getPoll)
	mux.HandleFunc("POST /polls/{id}/votes", app.vote)
	mux.HandleFunc("GET /healthz", health)

	httpServer := &http.Server{
		Addr:              ":8081",
		Handler:           otelhttp.NewHandler(mux, "votes"),
		ReadHeaderTimeout: 5 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		slog.Info("votes service listening", "address", httpServer.Addr)
		if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			slog.Error("serve", "error", err)
			os.Exit(1)
		}
	}()

	<-stop
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown", "error", err)
	}
}

func (s *server) createPoll(w http.ResponseWriter, r *http.Request) {
	var input pollInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validatePoll(&input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	id, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create poll")
		return
	}
	createdAt := time.Now().UTC()
	metadata := map[string]any{
		"question":     input.Question,
		"choice_count": len(input.Choices),
		"created_at":   createdAt.Format(time.RFC3339),
	}
	votes := make(map[string]any, len(input.Choices))
	for i, label := range input.Choices {
		metadata[fmt.Sprintf("choice:%d", i)] = label
		votes[strconv.Itoa(i)] = 0
	}

	pipe := s.valkey.TxPipeline()
	pipe.HSet(r.Context(), pollKey(id), metadata)
	pipe.HSet(r.Context(), voteKey(id), votes)
	pipe.ZAdd(r.Context(), "polls", redis.Z{Score: float64(createdAt.Unix()), Member: id})
	if _, err := pipe.Exec(r.Context()); err != nil {
		slog.ErrorContext(r.Context(), "create poll", "error", err)
		writeError(w, http.StatusServiceUnavailable, "data store unavailable")
		return
	}

	slog.InfoContext(r.Context(), "poll created", "poll_id", id)
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (s *server) listPolls(w http.ResponseWriter, r *http.Request) {
	ids, err := s.valkey.ZRevRange(r.Context(), "polls", 0, recentPollLimit-1).Result()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "data store unavailable")
		return
	}

	// ponytail: sequential reads are enough for 50 lab polls; pipeline if scale matters.
	polls := make([]poll, 0, len(ids))
	for _, id := range ids {
		item, err := s.loadPoll(r.Context(), id)
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "data store unavailable")
			return
		}
		polls = append(polls, item)
	}
	writeJSON(w, http.StatusOK, polls)
}

func (s *server) getPoll(w http.ResponseWriter, r *http.Request) {
	item, err := s.loadPoll(r.Context(), r.PathValue("id"))
	if errors.Is(err, redis.Nil) {
		writeError(w, http.StatusNotFound, "poll not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "data store unavailable")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *server) vote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, err := s.loadPoll(r.Context(), id)
	if errors.Is(err, redis.Nil) {
		writeError(w, http.StatusNotFound, "poll not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "data store unavailable")
		return
	}

	var input voteInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.Choice < 0 || input.Choice >= len(item.Choices) {
		writeError(w, http.StatusBadRequest, "choice is out of range")
		return
	}
	if err := s.valkey.HIncrBy(
		r.Context(), voteKey(id), strconv.Itoa(input.Choice), 1,
	).Err(); err != nil {
		writeError(w, http.StatusServiceUnavailable, "data store unavailable")
		return
	}

	slog.InfoContext(r.Context(), "vote recorded", "poll_id", id, "choice", input.Choice)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) loadPoll(ctx context.Context, id string) (poll, error) {
	metadata, err := s.valkey.HGetAll(ctx, pollKey(id)).Result()
	if err != nil {
		return poll{}, err
	}
	if len(metadata) == 0 {
		return poll{}, redis.Nil
	}
	count, err := strconv.Atoi(metadata["choice_count"])
	if err != nil || count < 2 {
		return poll{}, errors.New("invalid poll metadata")
	}
	votes, err := s.valkey.HGetAll(ctx, voteKey(id)).Result()
	if err != nil {
		return poll{}, err
	}

	item := poll{
		ID:        id,
		Question:  metadata["question"],
		Choices:   make([]choice, count),
		CreatedAt: metadata["created_at"],
	}
	for i := range count {
		value, err := strconv.ParseInt(votes[strconv.Itoa(i)], 10, 64)
		if err != nil {
			value = 0
		}
		item.Choices[i] = choice{Label: metadata[fmt.Sprintf("choice:%d", i)], Votes: value}
		item.TotalVotes += value
	}
	return item, nil
}

func validatePoll(input *pollInput) error {
	input.Question = strings.TrimSpace(input.Question)
	if len(input.Question) == 0 || len(input.Question) > 200 {
		return errors.New("question must contain between 1 and 200 characters")
	}

	clean := make([]string, 0, len(input.Choices))
	seen := make(map[string]bool, len(input.Choices))
	for _, raw := range input.Choices {
		label := strings.TrimSpace(raw)
		if label == "" {
			continue
		}
		if len(label) > 80 {
			return errors.New("choices must not exceed 80 characters")
		}
		key := strings.ToLower(label)
		if seen[key] {
			return errors.New("choices must be unique")
		}
		seen[key] = true
		clean = append(clean, label)
	}
	if len(clean) < 2 || len(clean) > 6 {
		return errors.New("a poll must contain between 2 and 6 choices")
	}
	input.Choices = clean
	return nil
}

func randomID() (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, value any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return errors.New("invalid JSON request")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request must contain one JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("write response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func pollKey(id string) string {
	return "poll:" + id
}

func voteKey(id string) string {
	return pollKey(id) + ":votes"
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
