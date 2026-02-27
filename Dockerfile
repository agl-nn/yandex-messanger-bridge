FROM golang:1.22-alpine

RUN apk add --no-cache git

WORKDIR /app

COPY . .

ENV GO111MODULE=on

RUN go mod download && \
    go build -o integrator ./cmd/integrator

EXPOSE 8080

CMD ["./integrator"]