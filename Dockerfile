FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Копируем go.mod
COPY go.mod ./

# Принудительно создаем go.sum и загружаем зависимости
RUN go mod download && \
    go mod verify && \
    go list -m all > /dev/null

# Копируем весь исходный код
COPY . .

# Проверяем наличие go.sum перед сборкой
RUN ls -la && \
    echo "=== go.mod content ===" && \
    cat go.mod && \
    echo "=== go.sum content ===" && \
    cat go.sum || echo "go.sum still empty"

# Собираем с явным указанием модуля
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -mod=readonly -o /app/bin/integrator ./cmd/integrator

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/bin/integrator /app/
COPY --from=builder /app/config /app/config
COPY --from=builder /app/migrations /app/migrations

EXPOSE 8080

CMD ["/app/integrator"]