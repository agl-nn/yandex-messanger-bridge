FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Копируем только go.mod
COPY go.mod ./

# Скачиваем зависимости
RUN go mod download

# Копируем остальной код
COPY . .

# Диагностика: показываем go.mod до добавления templ
RUN echo "=== go.mod BEFORE go get ===" && cat go.mod

# Добавляем templ в go.mod
RUN go get github.com/a-h/templ

# Диагностика: показываем go.mod после добавления templ
RUN echo "=== go.mod AFTER go get ===" && cat go.mod

# Теперь выполняем go mod tidy
RUN go mod tidy

# Диагностика: показываем go.mod после tidy
RUN echo "=== go.mod AFTER tidy ===" && cat go.mod

# Проверяем, есть ли templ в списке модулей
RUN go list -m all | grep templ || echo "❌ templ NOT FOUND in modules"

# Устанавливаем templ как бинарник
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