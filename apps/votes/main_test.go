package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidatePoll(t *testing.T) {
	input := pollInput{
		Question: "  Preferred runtime? ",
		Choices:  []string{"Containers", "", "WebAssembly"},
	}
	if err := validatePoll(&input); err != nil {
		t.Fatal(err)
	}
	if input.Question != "Preferred runtime?" || len(input.Choices) != 2 {
		t.Fatalf("unexpected normalized poll: %#v", input)
	}
}

func TestValidatePollRejectsDuplicates(t *testing.T) {
	input := pollInput{Question: "Tea?", Choices: []string{"Yes", "yes"}}
	if err := validatePoll(&input); err == nil || !strings.Contains(err.Error(), "unique") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestRandomID(t *testing.T) {
	first, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 16 || first == second {
		t.Fatalf("unexpected identifiers %q and %q", first, second)
	}
}

func TestDecodeJSONRejectsTrailingObject(t *testing.T) {
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{} {}`))
	if err := decodeJSON(httptest.NewRecorder(), request, &voteInput{}); err == nil {
		t.Fatal("expected trailing JSON object to be rejected")
	}
}
