# path: api/Dockerfile
# Build stage
FROM golang:alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /bin/observatory-api ./main.go

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /bin/observatory-api /usr/local/bin/observatory-api
USER 65534:65534
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/observatory-api"]

