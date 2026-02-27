FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Копируем только go.mod (go.sum может отсутствовать)
COPY go.mod ./

# Инициализируем модули и создаем go.sum
RUN go mod download && go mod tidy

# Копируем остальной код
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/integrator ./cmd/integrator

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/bin/integrator /app/
COPY --from=builder /app/config /app/config
COPY --from=builder /app/migrations /app/migrations

EXPOSE 8080

CMD ["/app/integrator"]