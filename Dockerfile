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


FROM alpine:latest

# Python
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    python3 \
    py3-pip \
    py3-numpy \
    py3-pandas \
    py3-scikit-learn

WORKDIR /app

COPY --from=builder /app/bin /app/bin
COPY config.yaml /app/config.yaml
COPY internal/ml/ /app/internal/ml/

CMD ["/app/bin/server"]
