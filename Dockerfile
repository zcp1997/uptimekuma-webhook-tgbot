# syntax=docker/dockerfile:1

FROM golang:1.23-alpine AS builder
WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o uptimekuma-webhook-tgbot ./...

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /app/uptimekuma-webhook-tgbot ./uptimekuma-webhook-tgbot

EXPOSE 8080
ENTRYPOINT ["./uptimekuma-webhook-tgbot"]
