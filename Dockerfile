FROM golang:1.24.9-alpine AS builder

WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ./cmd/server

FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/server /app/server
COPY configs /app/configs
COPY migrations /app/migrations
COPY swag /app/swag
COPY .env /app/.env

EXPOSE 8080

CMD ["/app/server"]
