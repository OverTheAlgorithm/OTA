# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o server .

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata python3 py3-pip \
    && pip3 install --no-cache-dir --break-system-packages googlenewsdecoder

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/scripts ./scripts

EXPOSE 8080

CMD ["./server"]
