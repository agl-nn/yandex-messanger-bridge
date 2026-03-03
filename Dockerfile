FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git wget

WORKDIR /app

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Создаем директории для статических файлов и скачиваем их
RUN mkdir -p /app/internal/web/static/js /app/internal/web/static/css && \
    wget -O /app/internal/web/static/js/htmx.min.js https://unpkg.com/htmx.org@1.9.10/dist/htmx.min.js && \
    wget -O /app/internal/web/static/css/tailwind.min.css https://cdn.jsdelivr.net/npm/tailwindcss@2.2.19/dist/tailwind.min.css

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