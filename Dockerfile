# Multi-stage build for Fixora
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o fixora ./cmd/fixora/main.go

# Production Stage
FROM alpine:3.20

# Install mandatory tools for Pre-Flight Validation Sandbox
RUN apk add --no-cache \
    kubectl \
    helm \
    ca-certificates \
    tzdata

WORKDIR /app
COPY --from=builder /app/fixora .

# Run as non-root for security
RUN adduser -D fixora
USER fixora

ENTRYPOINT ["./fixora"]
