FROM golang:1.26-alpine AS builder

WORKDIR /build
COPY pkg/cantonledger ./pkg/cantonledger
COPY listener ./listener

WORKDIR /build/listener
RUN go mod download && go build -o /listener .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /listener .

CMD ["./listener"]
