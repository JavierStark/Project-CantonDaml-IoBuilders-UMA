FROM golang:1.26-alpine AS builder

WORKDIR /build
COPY pkg/cantonledger ./pkg/cantonledger
COPY backend ./backend

WORKDIR /build/backend
RUN go mod download && go build -o /server .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /server .

EXPOSE 8080

CMD ["./server"]
