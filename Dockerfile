FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git

# Устанавливаем templ
RUN go install github.com/a-h/templ/cmd/templ@latest

WORKDIR /app

COPY . .

# Генерируем Go код из templ шаблонов
RUN templ generate

RUN go mod tidy && \
    go build -o integrator ./cmd/integrator

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/integrator /app/
COPY --from=builder /app/config /app/config
COPY --from=builder /app/migrations /app/migrations

EXPOSE 8080

CMD ["./integrator"]