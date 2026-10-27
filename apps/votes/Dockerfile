FROM dhi.io/golang:1.26-debian13-dev AS build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /votes .

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /votes /votes
USER 65532:65532

EXPOSE 8081
CMD ["/votes"]

