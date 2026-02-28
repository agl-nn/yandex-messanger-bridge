FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Копируем только go.mod
COPY go.mod ./

# Скачиваем все зависимости
RUN go mod download && go mod tidy

# Проверяем go.mod до добавления templ
RUN echo "=== go.mod BEFORE adding templ ===" && cat go.mod

# Добавляем templ в go.mod
RUN go get github.com/a-h/templ

# Проверяем go.mod после добавления templ
RUN echo "=== go.mod AFTER adding templ ===" && cat go.mod

# Устанавливаем templ как бинарник
RUN go install github.com/a-h/templ/cmd/templ@latest

# Копируем остальной код
COPY . .

# Еще раз обновляем зависимости
RUN go mod tidy

# Проверяем финальный go.mod
RUN echo "=== go.mod FINAL ===" && cat go.mod

# Проверяем, что templ виден в зависимостях
RUN go list -m all | grep templ || echo "templ not found in modules!"

# Генерируем шаблоны
RUN templ generate

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