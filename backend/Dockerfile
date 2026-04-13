FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o api ./cmd/api

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /build/api .
COPY --from=builder /build/migrations ./migrations
COPY --from=builder /build/seed ./seed

EXPOSE 8080

CMD ["./api"]
