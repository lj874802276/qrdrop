FROM golang:1.22-alpine AS builder
# QRDrop uses a pure-Go SQLite driver (modernc.org/sqlite), so CGO is not
# needed. Disabling it yields a fully static binary and a faster build.
ENV CGO_ENABLED=0
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o qrdrop .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/qrdrop .
EXPOSE 8080
CMD ["./qrdrop"]
