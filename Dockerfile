# Этап сборки
FROM golang:1.22-alpine AS builder

# Устанавливаем зависимости для сборки
RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

# Копируем модули
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем бинарник
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/integrator ./cmd/integrator

# Этап выполнения
FROM alpine:latest

# Устанавливаем CA сертификаты
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Копируем бинарник
COPY --from=builder /app/bin/integrator /app/integrator

# Копируем конфиги и миграции
COPY --from=builder /app/config /app/config
COPY --from=builder /app/migrations /app/migrations

# Создаем непривилегированного пользователя
RUN adduser -D -u 1000 appuser
USER appuser

# Экспонируем порт
EXPOSE 8080

# Запускаем
CMD ["/app/integrator"]