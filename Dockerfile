# build stage
FROM golang:1.26.3 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o bus-app .

# runtime stage
FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/bus-app .

EXPOSE 8080

CMD ["./bus-app"]
