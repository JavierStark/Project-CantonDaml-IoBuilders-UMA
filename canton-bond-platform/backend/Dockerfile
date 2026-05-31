FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod ./
COPY . .

RUN go build -o /server ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /server .

EXPOSE 8080

CMD ["./server"]
