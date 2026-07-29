FROM golang:1.26

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o votes .

EXPOSE 8081
CMD ["./votes"]

