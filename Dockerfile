FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Копируем только go.mod
COPY go.mod ./

# Скачиваем все зависимости
RUN go mod download && go mod tidy

# УСТАНАВЛИВАЕМ TEMPL КАК БИНАРНИК (не через go run)
RUN go install github.com/a-h/templ/cmd/templ@latest

# Копируем остальной код
COPY . .

# Еще раз обновляем зависимости
RUN go mod tidy

# Генерируем шаблоны через установленный бинарник
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