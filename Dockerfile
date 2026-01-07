FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY . .
RUN go build -o bksyrss -ldflags="-w -s" .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/bksyrss .
VOLUME ["/data"]
ENTRYPOINT ["./bksyrss"]
