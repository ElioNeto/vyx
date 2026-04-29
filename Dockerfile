# syntax=docker/dockerfile:1

# ─────────────────────────────────────────
# Stage 1: Build
# ─────────────────────────────────────────
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build deps
RUN apk add --no-cache git ca-certificates

# Download dependencies first (layer cache)
COPY go.work go.work.sum ./
COPY go.mod ./
COPY cmd/vyx/go.mod cmd/vyx/go.sum* ./cmd/vyx/

RUN go work sync

# Copy source
COPY . .

# Build the main binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/vyx ./cmd/vyx

# ─────────────────────────────────────────
# Stage 2: Runtime
# ─────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

COPY --from=builder /out/vyx /vyx

ENTRYPOINT ["/vyx"]
