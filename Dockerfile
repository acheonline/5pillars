FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CGO_CFLAGS=-D_LARGEFILE64_SOURCE
RUN go build main.go

FROM alpine

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -g 1000 appuser && \
    adduser -u 1000 -S -G appuser appuser

RUN mkdir -p /data && chown -R appuser:appuser /data

COPY --from=builder --chown=appuser:appuser /app/main /app/main

WORKDIR /app

USER appuser

EXPOSE 8080

CMD ["./main"]