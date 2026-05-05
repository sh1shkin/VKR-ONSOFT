FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/api ./cmd/api

FROM alpine:3.20

WORKDIR /app
COPY --from=builder /out/api /app/api

EXPOSE 8080

CMD ["/app/api"]