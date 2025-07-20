
FROM golang:1.24.5-alpine AS builder


RUN apk add --no-cache git

WORKDIR /app


COPY go.mod go.sum ./
RUN go mod download


COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o eth-validator-api ./cmd/api


FROM alpine:3.18


RUN apk add --no-cache ca-certificates

WORKDIR /root/


COPY --from=builder /app/eth-validator-api .
COPY config.json .

EXPOSE 8080

ENTRYPOINT ["./eth-validator-api"]
