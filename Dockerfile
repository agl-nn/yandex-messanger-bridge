FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем остальной код
COPY . .

# Устанавливаем templ
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1001

# Генерируем шаблоны
RUN templ generate

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o integrator ./cmd/integrator

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/integrator /app/
COPY --from=builder /app/config /app/config
COPY --from=builder /app/migrations /app/migrations
COPY --from=builder /app/internal/web/static /app/internal/web/static

EXPOSE 8080

CMD ["./integrator"]