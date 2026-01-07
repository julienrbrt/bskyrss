FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY . .
RUN go install -ldflags="-w -s" .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /go/bin/bskyrss .
VOLUME ["/data"]
ENTRYPOINT ["./bskyrss"]
