FROM golang:1.25.13-alpine AS builder

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /gate .

FROM alpine:3.23

RUN apk upgrade --no-cache && apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /gate .

RUN addgroup -S gate && adduser -S gate -G gate

USER gate

EXPOSE 8080

CMD ["/app/gate"]
