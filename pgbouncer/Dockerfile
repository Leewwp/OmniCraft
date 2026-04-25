FROM alpine:3.19

RUN apk add --no-cache pgbouncer

RUN addgroup -S pgbouncer && adduser -S -D -H -G pgbouncer pgbouncer

USER pgbouncer

EXPOSE 5432

CMD ["pgbouncer", "/etc/pgbouncer/pgbouncer.ini"]