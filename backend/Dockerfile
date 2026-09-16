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

ENV APP_ENV=production
ENV PORT=8080

USER app
EXPOSE 8080

ENTRYPOINT ["/app/server"]
