FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN go build -o /out/alert-sink .

FROM alpine:3.19
RUN adduser -D -u 10001 alert-sink \
  && mkdir -p /events && chown alert-sink:alert-sink /events
USER alert-sink
COPY --from=build /out/alert-sink /usr/local/bin/alert-sink
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/alert-sink"]
