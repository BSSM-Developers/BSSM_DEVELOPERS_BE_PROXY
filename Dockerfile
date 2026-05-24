FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o proxy ./cmd/...

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /app/proxy .
EXPOSE 8080
ENTRYPOINT ["/app/proxy"]
