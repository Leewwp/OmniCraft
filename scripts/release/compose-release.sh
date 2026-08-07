#!/usr/bin/env bash
# Shared real-release operations for deploy.sh and rollback.sh. The caller must
# set RELEASE_ENV_FILE, RELEASE_COMPOSE_FILE, RELEASE_OVERRIDE_FILE,
# RELEASE_REPORT_DIR and RELEASE_DOCKER_BIN before sourcing this file.

release_compose() {
  "$RELEASE_DOCKER_BIN" compose --env-file "$RELEASE_ENV_FILE" \
    -f "$RELEASE_COMPOSE_FILE" -f "$RELEASE_OVERRIDE_FILE" "$@"
}

render_release_override() {
  local manifest="$1" output="$2" include_migrate="${3:-1}"
  python3 - "$manifest" "$output" "$include_migrate" <<'PY'
import json
import sys

manifest_path, output_path, include_migrate = sys.argv[1:4]
with open(manifest_path, encoding="utf-8") as f:
    manifest = json.load(f)
images = manifest["images"]
services = [("backend", images["backend"]["ref"]),
            ("frontend", images["frontend"]["ref"])]
if include_migrate == "1":
    services.insert(1, ("migrate", images["backend"]["ref"]))
with open(output_path, "w", encoding="utf-8") as f:
    f.write("services:\n")
    for service, image in services:
        f.write(f"  {service}:\n")
        f.write(f"    image: {image}\n")
        f.write("    build: null\n")
PY
}

wait_for_release_health() {
  local timeout="${OMNICRAFT_HEALTH_TIMEOUT_SEC:-300}"
  local services=(backend frontend nginx)
  local started
  started="$(date +%s)"
  while :; do
    local ready=1
    for service in "${services[@]}"; do
      local container status
      container="$(release_compose ps -q "$service")"
      if [ -z "$container" ]; then
        ready=0
        continue
      fi
      status="$("$RELEASE_DOCKER_BIN" inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container" 2>/dev/null || true)"
      if [ "$status" != "healthy" ]; then
        ready=0
      fi
      printf '%s=%s\n' "$service" "$status" >> "$RELEASE_REPORT_DIR/readiness.log"
    done
    if [ "$ready" -eq 1 ]; then
      return 0
    fi
    if [ $(( $(date +%s) - started )) -ge "$timeout" ]; then
      echo "release: services did not become healthy within ${timeout}s" >&2
      return 1
    fi
    sleep 2
  done
}

run_release_smoke() {
  local smoke_url="${OMNICRAFT_SMOKE_URL:-}"
  local timeout="${OMNICRAFT_SMOKE_TIMEOUT_SEC:-10}"
  if [ -z "$smoke_url" ]; then
    echo "release: OMNICRAFT_SMOKE_URL is required for a real deploy" >&2
    return 1
  fi
  curl --fail --silent --show-error --max-time "$timeout" "$smoke_url" > "$RELEASE_REPORT_DIR/smoke-response.txt"
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
    return
  fi
  shasum -a 256 "$1" | awk '{print $1}'
}

release_env_value() {
  local key="$1"
  awk -F= -v key="$key" '$1 == key {sub(/^[^=]*=/, ""); print; exit}' "$RELEASE_ENV_FILE"
}

run_release_backup() {
  local stamp backup_file user db
  stamp="$(date -u +%Y%m%dT%H%M%SZ)"
  backup_file="$RELEASE_REPORT_DIR/postgres-${stamp}.custom"
  user="${POSTGRES_USER:-$(release_env_value POSTGRES_USER)}"
  db="${POSTGRES_DB:-$(release_env_value POSTGRES_DB)}"
  user="${user:-omnicraft}"
  db="${db:-omnicraft}"
  release_compose exec -T postgres pg_dump -U "$user" -d "$db" --format=custom --no-owner --no-acl > "$backup_file"
  local checksum
  checksum="$(sha256_file "$backup_file")"
  python3 - "$RELEASE_REPORT_DIR/backup-manifest.json" "$backup_file" "$checksum" "$db" <<'PY'
import datetime
import json
import sys

path, backup_file, checksum, db = sys.argv[1:5]
with open(path, "w", encoding="utf-8") as f:
    json.dump({
        "id": checksum,
        "status": "ok",
        "backup_file": backup_file,
        "checksum_sha256": checksum,
        "database": db,
        "created_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    }, f, indent=2)
    f.write("\n")
PY
  printf '%s\n' "$checksum"
}

run_release_migration() {
  # Keep the successful one-shot container so backend's
  # service_completed_successfully dependency is observable by Compose.
  release_compose up --no-build --force-recreate migrate > "$RELEASE_REPORT_DIR/migration.log" 2>&1
  # The single-server compose mounts the project-level artifacts directory at
  # /app/evidence. Copy the migration summary into the Ops report directory so
  # deployment archiving cannot silently omit the applied migration evidence.
  local project_artifact_dir migration_summary
  project_artifact_dir="$(cd "$(dirname "$RELEASE_COMPOSE_FILE")" 2>/dev/null && pwd || true)"
  migration_summary="$project_artifact_dir/artifacts/migration-summary.json"
  if [ -f "$migration_summary" ] && [ "$migration_summary" != "$RELEASE_REPORT_DIR/migration-summary.json" ]; then
    cp "$migration_summary" "$RELEASE_REPORT_DIR/migration-summary.json"
  fi
}

deploy_release_images() {
  release_compose pull backend frontend migrate > "$RELEASE_REPORT_DIR/compose-pull.log" 2>&1
  release_compose up -d --no-build postgres redis pgbouncer > "$RELEASE_REPORT_DIR/compose-dependencies.log" 2>&1
}

activate_release_images() {
  release_compose up -d --no-build --force-recreate backend frontend nginx > "$RELEASE_REPORT_DIR/compose-activate.log" 2>&1
}

rollback_release_images() {
  release_compose pull backend frontend > "$RELEASE_REPORT_DIR/compose-rollback-pull.log" 2>&1
  release_compose up -d --no-build --force-recreate backend frontend nginx > "$RELEASE_REPORT_DIR/compose-rollback-activate.log" 2>&1
}
