FROM alpine:3.23

RUN apk add --no-cache pgbouncer postgresql-client

RUN addgroup -S pgbouncer && adduser -S -D -H -G pgbouncer pgbouncer

USER pgbouncer

EXPOSE 5432

CMD ["pgbouncer", "/etc/pgbouncer/pgbouncer.ini"]