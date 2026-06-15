# Build context is the services/admin/backend/ directory (set by docker-compose context: ..)

# ---- build stage ----
FROM golang:alpine AS builder

WORKDIR /app

# Cache dependency downloads as a separate layer
COPY go.mod go.sum ./
RUN go mod download

# Install code generators as their own cached layer. Versions are pinned to
# match the runtime packages in go.mod — bump them together when upgrading deps.
# protoc-gen-go version mirrors google.golang.org/protobuf in go.mod.
# protoc-gen-go-grpc version mirrors google.golang.org/grpc/cmd/protoc-gen-go-grpc.
ARG TEMPL_VERSION=v0.3.1020
ARG PROTOC_GEN_GO_VERSION=v1.36.11
ARG PROTOC_GEN_GO_GRPC_VERSION=v1.6.2
RUN go install github.com/a-h/templ/cmd/templ@${TEMPL_VERSION} && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@${PROTOC_GEN_GO_VERSION} && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@${PROTOC_GEN_GO_GRPC_VERSION}

# Install protoc (the protobuf compiler that drives the Go plugins above).
RUN apk add --no-cache protobuf

COPY . .

# Regenerate *_templ.go and gRPC bindings from their sources so the image
# cannot ship stale generated files from a developer's working tree.
RUN templ generate
RUN protoc \
      --proto_path=proto \
      --go_out=. \
      --go_opt=module=charity-chest/services/admin/backend \
      --go-grpc_out=. \
      --go-grpc_opt=module=charity-chest/services/admin/backend \
      admin/internal/v1/internal.proto

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/server . && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/seed-root ./cmd/seed-root

# ---- runtime stage ----
FROM alpine:3.23

# ca-certificates: needed for HTTPS calls to Google OAuth endpoints
# tzdata: consistent timezone handling
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/server      ./server
COPY --from=builder /bin/seed-root   ./seed-root
COPY --from=builder /app/migrations  ./migrations
COPY ./aws/postgres/global-bundle.pem ./
COPY ./.docker-dev/entry-point.sh ./entry-point.sh
RUN chmod +x ./entry-point.sh

EXPOSE 8080

ENTRYPOINT ["./entry-point.sh"]
