# OpenShip primary-source research

Created: 2026-07-25

Expected expiry: 2026-09-25

## Scope

Assessment notes gathered only from the upstream OpenShip GitHub repository and its GitHub API metadata: <https://github.com/oblien/openship>.

## Verified facts

- OpenShip describes itself as an Apache-2.0, open-source self-hostable deployment platform with built-in CI/CD. It accepts a GitHub repository, local folder, or pre-built artifact; then detects configuration, builds, runs, routes, and TLS-terminates the app. Interfaces are a desktop app, web dashboard, CLI, REST API, and MCP endpoint. [README](https://github.com/oblien/openship/blob/main/README.md#how-it-works)
- Its advertised operational scope includes Git push deployments, preview/staging/production flows, rollback, Docker Compose, automatic Let's Encrypt certificates, domains, scheduled database/volume backups, real-time logs/metrics, and a catalog of common self-hosted apps. [README features](https://github.com/oblien/openship/blob/main/README.md#features)
- For an always-on self-hosted Linux instance, its default Docker Compose mode runs Postgres, Redis, API, dashboard and an OpenResty edge that occupies host ports 80 and 443. Docker Compose self-hosting is Linux-only because the edge uses host networking. [README installation](https://github.com/oblien/openship/blob/main/README.md#team--always-on--self-hosted-server) [reference compose file](https://github.com/oblien/openship/blob/main/docker/docker-compose.yml)
- In that Compose mode the API mounts `/var/run/docker.sock` and creates/manages application containers using the host Docker daemon. The upstream file explicitly calls this host-privileged and says to use it only on a trusted host. [reference compose file](https://github.com/oblien/openship/blob/main/docker/docker-compose.yml)
- Desktop mode is designed for a solo operator: its local control plane manages remote servers over SSH. Upstream says it is insufficient for push-to-deploy, team access, or applications hosted by the controller itself, which require an always-on public control plane. [installation guide](https://github.com/oblien/openship/blob/main/docs/installation.md)
- Latest GitHub release when checked was v0.3.0 (published 2026-07-22), with desktop/server assets and SHA-256 sidecars. GitHub metadata shows the repository was created 2026-03-05, and it remains in the 0.x release series. [release](https://github.com/oblien/openship/releases/tag/v0.3.0) [repository API](https://api.github.com/repos/oblien/openship)
- Upstream calls the core "production-ready" but lists multi-node clustering, load-balancing UI, private networking, advanced monitoring, and visual CI/CD as future work. Its own documentation notes are still being filled out. [README status](https://github.com/oblien/openship/blob/main/README.md#status) [README interfaces](https://github.com/oblien/openship/blob/main/README.md#interfaces)

## Implication for an already deployed solo project

It can replace repeated manual build/SSH/Docker/reverse-proxy/certificate work with a single control plane, so it becomes more compelling with several services, frequent releases, staging/preview needs, or a need for visible rollbacks/backups.

It is not an unqualified drop-in upgrade for a stable production server: it adds a privileged control plane, Postgres, Redis, public ports and an edge proxy; the controller's Docker-socket access has host-level impact. Its young 0.x status also warrants a non-critical workload or separate VPS trial before it manages the main production workload.

## Comparison with OmniCraft's present single-server path

OmniCraft's documented beta deployment is already a conventional, single-host Docker Compose stack: Next.js frontend, Go API, PostgreSQL, PgBouncer, Redis and Nginx; only Nginx exposes 80/443. It provisions TLS with host Certbot and explicitly keeps production secrets in a server-side `.env`. [OmniCraft architecture](../../architecture.md#82-docker-compose-服务清单) [single-server runbook](../deploy/single-server-beta-runbook.md)

| Concern | Current OmniCraft path | What OpenShip would change |
|---|---|---|
| Release process | Build and start the known Compose stack from the server runbook. | Adds repository-driven detection/build/deploy, GUI/CLI logs, webhook-triggered deployments and rollback metadata. |
| Public ingress | Existing Nginx and Certbot own ports 80/443 and the application/API domains. | Its Compose edge also owns 80/443, so it cannot be installed alongside the current ingress without an explicit migration or separate host/IP. [OpenShip compose](https://github.com/oblien/openship/blob/main/docker/docker-compose.yml) |
| State and secrets | One application stack; its documented credentials remain outside Git. | Adds another Postgres, Redis, dashboard/API and a controller that must safeguard deployment credentials; its API receives the host Docker socket. |
| Operations burden | You own Compose, Nginx, certificate renewal, deployment and backup scripts. | Less repetitive deploy work, but another production system, upgrade stream, backup/restore procedure and security boundary to operate. |
| Failure domain | Deployment tooling is largely separate from the running app. | A compromise or misconfiguration of the socket-enabled controller can affect every Docker workload on that host. |

## Recommendation

For the current stage, do **not** make OpenShip the primary production deployment path yet. OmniCraft is a single application already designed for a Docker Compose + Nginx single-server release path, and OpenShip's main short-term benefit (automating frequent multi-service releases) does not outweigh the ingress migration and privileged-controller risk for one stable workload.

Reconsider it when releases become frequent enough that manual deployment is the bottleneck, or when you operate several independent apps/environments. The low-risk evaluation is a separate VPS (or a disposable non-production service): pin a specific OpenShip release, verify its backup/restore and rollback behaviour, keep OmniCraft's existing server untouched, and only then decide whether a controlled ingress migration is worthwhile.
