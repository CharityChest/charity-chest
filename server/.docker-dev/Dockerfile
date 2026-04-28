# Build context is the server/ directory (set by docker-compose context: ..)

# ---- build stage ----
FROM golang:alpine AS builder

WORKDIR /app

# Cache dependency downloads as a separate layer
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/server . && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/seed-root ./cmd/seed-root

# ---- runtime stage ----
FROM alpine:latest

# ca-certificates: needed for HTTPS calls to Google OAuth endpoints
# tzdata: consistent timezone handling
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/server      ./server
COPY --from=builder /bin/seed-root   ./seed-root
COPY --from=builder /app/migrations  ./migrations

EXPOSE 8080

ENTRYPOINT ["./server"]