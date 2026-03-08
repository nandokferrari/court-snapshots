FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o court-snapshots .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/court-snapshots /usr/local/bin/
CMD ["court-snapshots"]
