# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache deps first.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build static binary.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 app

WORKDIR /app
COPY --from=builder /out/server /app/server

# Create storage dir owned by app so non-root runtime can write uploads.
RUN mkdir -p /data/storage && chown -R 10001:10001 /data

USER app
EXPOSE 8080

ENTRYPOINT ["/app/server"]