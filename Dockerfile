FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build Go services
RUN go build -o /app/bin/server ./cmd/server
RUN go build -o /app/bin/collect ./cmd/collect
RUN go build -o /app/bin/store ./cmd/store
RUN go build -o /app/bin/detect ./cmd/detect
RUN go build -o /app/bin/seed ./cmd/seed


FROM python:3.11-slim

# Install ca-certificates for HTTPS requests
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/bin /app/bin
COPY config.yaml /app/config.yaml
COPY internal/ml/ /app/internal/ml/
COPY requirements.txt /app/requirements.txt

# Install Python dependencies (will use pre-built wheels)
RUN pip install --no-cache-dir -r requirements.txt

CMD ["/app/bin/server"]
