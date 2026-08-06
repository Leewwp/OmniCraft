# Build stage
FROM golang:1.25.12-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

ENV GOPROXY=https://goproxy.cn,direct \
	GOSUMDB=sum.golang.google.cn

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./cmd/server/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/migrate ./cmd/migrate

# Final stage
FROM alpine:3.23

ARG VERSION=dev
ARG COMMIT=unknown

LABEL org.opencontainers.image.title="OmniCraft backend" \
	org.opencontainers.image.description="OmniCraft Go/Gin API server" \
	org.opencontainers.image.source="https://github.com/omnicraft/omnicraft" \
	org.opencontainers.image.version="$VERSION" \
	org.opencontainers.image.revision="$COMMIT"

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/migrate .
COPY --from=builder /app/config.yaml .
COPY --from=builder /app/migrations ./migrations

RUN addgroup -S omnicraft && adduser -S omnicraft -G omnicraft \
	&& mkdir -p /app/evidence /var/lib/omnicraft/metrics \
	&& chown -R omnicraft:omnicraft /app /var/lib/omnicraft

USER omnicraft

EXPOSE 8080

CMD ["./server"]
