# syntax=docker/dockerfile:1.4
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git gcc musl-dev ca-certificates && update-ca-certificates
WORKDIR /app

ENV CGO_ENABLED=1

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o appchain ./cmd/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/appchain .
EXPOSE 8080
ENTRYPOINT ["./appchain"]
