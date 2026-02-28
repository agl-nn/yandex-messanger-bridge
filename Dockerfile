FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Копируем только go.mod
COPY go.mod ./

# Первичная загрузка зависимостей
RUN go mod download

# Копируем остальной код
COPY . .

# Полностью обновляем зависимости и создаем go.sum
RUN go mod tidy

# Устанавливаем templ в модуль
RUN go get github.com/a-h/templ

# Генерируем Go код из templ шаблонов
RUN go run github.com/a-h/templ/cmd/templ generate

# Собираем приложение
RUN go build -o integrator ./cmd/integrator

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/integrator /app/
COPY --from=builder /app/config /app/config
COPY --from=builder /app/migrations /app/migrations

EXPOSE 8080

CMD ["./integrator"]