# Build stage
FROM golang:1.25.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o fixora ./cmd/fixora

# Final stage
FROM gcr.io/distroless/static-debian11

WORKDIR /

COPY --from=builder /app/fixora .

USER 65532:65532

ENTRYPOINT ["/fixora"]
