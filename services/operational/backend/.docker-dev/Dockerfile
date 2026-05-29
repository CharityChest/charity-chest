# Build context is the services/operational/backend/ directory
# (set by docker-compose context: ..).

# ---- build stage ----
FROM golang:alpine AS builder

WORKDIR /app

# Cache dependency downloads as a separate layer.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/server .

# ---- runtime stage ----
FROM alpine:3.23

# ca-certificates: needed for HTTPS calls to Google's tokeninfo and the
#   admin backend over TLS in non-dev environments.
# tzdata: consistent timezone handling.
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/server     ./server
COPY --from=builder /app/migrations ./migrations
COPY ./.docker-dev/entry-point.sh   ./entry-point.sh
RUN chmod +x ./entry-point.sh

EXPOSE 8081

ENTRYPOINT ["./entry-point.sh"]
