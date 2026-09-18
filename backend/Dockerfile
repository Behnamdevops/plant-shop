# syntax=docker/dockerfile:1

# ---- Build stage ---------------------------------------------------------
# Builds a static, small Go binary. The Go toolchain never ships in the
# final image.
FROM golang:1.26-alpine AS build

WORKDIR /src

# Cache dependency downloads separately from source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 produces a statically linked binary that runs on the
# minimal "scratch"-like distroless/alpine runtime without libc issues.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server .

# ---- Runtime stage --------------------------------------------------------
FROM alpine:3.20 AS runtime

# ca-certificates is required for TLS connections (e.g. DATABASE_URL with
# sslmode=require against a managed Postgres provider).
RUN apk add --no-cache ca-certificates && \
    addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=build /out/server /app/server
# Migrations are applied by the same binary in production (APP_ENV=production)
# before the HTTP server starts, or manually via `go run ./cmd/migrate`.
COPY migrations /app/migrations

# Admin-uploaded product images are written here (see UPLOAD_DIR). In
# docker-compose.prod.yml this path is backed by a dedicated named volume
# so uploads survive container recreation. Owned by the unprivileged "app"
# user the server runs as.
RUN mkdir -p /app/data/uploads/products && chown -R app:app /app/data

ENV APP_ENV=production
ENV PORT=8080
ENV UPLOAD_DIR=/app/data/uploads/products

USER app
EXPOSE 8080

ENTRYPOINT ["/app/server"]
