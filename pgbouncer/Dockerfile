FROM alpine:3.23

# Upgrade bundled packages before installing: apk add alone keeps the base
# image's preinstalled libcrypto3/libssl3 at their bundled version (apk does
# not upgrade installed deps), which shipped a vulnerable openssl that the
# trivy-image gate correctly blocked (CVE-2026-14456, #497). The backend
# image already follows this pattern.
RUN apk upgrade --no-cache && apk add --no-cache pgbouncer postgresql-client

RUN addgroup -S pgbouncer && adduser -S -D -H -G pgbouncer pgbouncer

USER pgbouncer

EXPOSE 5432

CMD ["pgbouncer", "/etc/pgbouncer/pgbouncer.ini"]