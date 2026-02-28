FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Копируем только go.mod (go.sum пока нет)
COPY go.mod ./

# Скачиваем зависимости и создаем go.sum
RUN go mod download && \
    go mod tidy && \
    go get github.com/a-h/templ

# Теперь копируем остальной код
COPY . .

# Генерируем Go код из templ шаблонов
RUN go run github.com/a-h/templ/cmd/templ generate

RUN go build -o integrator ./cmd/integrator

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/integrator /app/
COPY --from=builder /app/config /app/config
COPY --from=builder /app/migrations /app/migrations

EXPOSE 8080

CMD ["./integrator"]