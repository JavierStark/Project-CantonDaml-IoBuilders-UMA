FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod ./
COPY . .

RUN go build -o /listener .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /listener .

CMD ["./listener"]
