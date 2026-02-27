FROM golang:1.22-alpine

RUN apk add --no-cache git

WORKDIR /app

# Копируем всё
COPY . .

# Включаем модули и прокси
ENV GO111MODULE=on
ENV GOPROXY=https://proxy.golang.org,direct

# Сначала обновим зависимости и создадим go.sum
RUN go mod tidy

# Теперь собираем
RUN go build -o integrator ./cmd/integrator

EXPOSE 8080

CMD ["./integrator"]