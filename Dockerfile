FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Копируем только go.mod
COPY go.mod ./

# Скачиваем зависимости
RUN go mod download

# Копируем остальной код
COPY . .

# Добавляем templ в go.mod (ВАЖНО!)
RUN go get github.com/a-h/templ

# Устанавливаем templ как бинарник
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1001

# Генерируем шаблоны (ТЕПЕРЬ templ есть в go.mod)
RUN templ generate

# Теперь выполняем go mod tidy (после генерации шаблонов)
RUN go mod tidy

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