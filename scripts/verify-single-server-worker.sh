#!/usr/bin/env bash
# F3 #988 worker 服务定义验证（single-server compose）。
#
# 用法：
#   scripts/verify-single-server-worker.sh --check   静态：compose config 有效
#                                                   + base 文件含 worker 定义
#                                                   + lean override 合并健康
#   scripts/verify-single-server-worker.sh --local   动态：最小本地栈
#                                                   （postgres/redis/pgbouncer/
#                                                   migrate/worker）起栈 →
#                                                   投递一条通知事件 → 断言落库
#                                                   + consumer lag 回 0 + worker
#                                                   容器 healthy。
#
# --local 前置：docker；pgvector/redis 镜像（compose 钉的 digest）与本地
# pgbouncer 镜像、backend 构建基础镜像已在本地缓存（离线可跑）；脚本用临时
# compose overlay 把宿主路径（artifacts/metrics/config_override）指到 /tmp。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_BASE="$ROOT/docs/deploy/docker-compose.single-server.yml"
COMPOSE_LEAN="$ROOT/docs/deploy/docker-compose.lean.yml"
MODE="${1:---check}"

# compose 插值必需变量（:? 门）——静态/动态本地验证都用占位值，不涉真实密钥。
ENV_FILE="$(mktemp)"
trap 'rm -f "$ENV_FILE"' EXIT
cat > "$ENV_FILE" <<'EOF'
POSTGRES_PASSWORD=local-verify-pg-pass
REDIS_PASSWORD=local-verify-redis-pass
REDIS_DB=0
PGBOUNCER_IMAGE=edoburu/pgbouncer@sha256:4c1ca296ef525f108f5d3552cc337c0c09587cf8dae7f0067fd93349e47dc1cd
METRICS_TEXTFILE_DIR=/tmp/omnicraft-f3-verify/metrics
OBS_LOG_DIR=/tmp/omnicraft-f3-verify/logs
OBS_GATE_TOKEN=local-verify-gate-token
ALERTMANAGER_CONFIG_FILE=/tmp/omnicraft-f3-verify/alertmanager.yml
EOF

fail() { echo "FAIL  $*" >&2; exit 1; }
pass() { echo "PASS  $*"; }

DOTENV="$ROOT/.env"

check_static() {
  # env_file: .env 在 config 校验时也须存在——临时 dummy，退出即清。
  : > "$DOTENV"
  trap 'rm -f "$ENV_FILE" "$DOTENV"' EXIT

  local cfg
  cfg="$(mktemp)"
  if ! docker compose --project-directory "$ROOT" -f "$COMPOSE_BASE" --env-file "$ENV_FILE" config > "$cfg" 2>/dev/null; then
    fail "base compose config 解析失败"
  fi
  pass "base compose config 有效"

  grep -q '^  worker:' "$cfg" || fail "base compose 缺 worker 服务定义（#988：worker 是 ADR0005 独立进程，必须随基础栈部署，不能只活在 lean override）"
  pass "base compose 含 worker 服务定义"

  # 定义形态断言：与 backend 同源构建、命令切 worker、依赖/重启/健康检查齐备。
  python3 - "$cfg" <<'PY'
import sys, yaml
cfg = yaml.safe_load(open(sys.argv[1]))
w = cfg["services"].get("worker") or sys.exit("worker service missing")
errs = []
if not str(w.get("build", {}).get("context", "")).rstrip("/").endswith("/backend"):
    errs.append("build.context 不是 ./backend（config 输出为绝对路径，按尾段断言）")
if w.get("command") not in (["./worker"], "./worker"):
    errs.append("command 未切到 ./worker")
deps = w.get("depends_on", {})
for svc, cond in (("migrate", "service_completed_successfully"),
                  ("postgres", "service_healthy"),
                  ("pgbouncer", "service_healthy"),
                  ("redis", "service_healthy")):
    if not isinstance(deps.get(svc), dict) or deps[svc].get("condition") != cond:
        errs.append(f"depends_on.{svc} 期望 {cond}")
if w.get("restart") != "unless-stopped":
    errs.append("restart != unless-stopped")
if not w.get("healthcheck"):
    errs.append("缺 healthcheck（验收：worker 容器 healthy）")
env = w.get("environment", {})
for key in ("DB_DSN", "REDIS_ADDR", "REDIS_PASSWORD"):
    if key not in env:
        errs.append(f"environment 缺 {key}")
if errs:
    sys.exit("worker 定义形态问题: " + "; ".join(errs))
PY
  pass "worker 定义形态（构建/命令/依赖/重启/健康检查/env）"

  # lean override 合并：worker 仍在且资源帽 256m 生效；merged config 有效。
  local merged
  merged="$(mktemp)"
  if ! docker compose --project-directory "$ROOT" -f "$COMPOSE_BASE" -f "$COMPOSE_LEAN" --env-file "$ENV_FILE" config > "$merged" 2>/dev/null; then
    fail "base+lean 合并 config 解析失败"
  fi
  python3 - "$merged" <<'PY'
import sys, yaml
cfg = yaml.safe_load(open(sys.argv[1]))
w = cfg["services"].get("worker") or sys.exit("merged config 缺 worker")
mem = w.get("deploy", {}).get("resources", {}).get("limits", {}).get("memory")
if str(mem) not in ("256m", "268435456"):
    sys.exit(f"lean 资源帽未生效: {mem}")
PY
  pass "base+lean 合并有效，worker lean 资源帽 256m"

  echo "static: ALL PASS"
}

WORK_DIR=/tmp/omnicraft-f3-verify

check_local() {
  local work="$WORK_DIR"
  rm -rf "$work"; mkdir -p "$work/metrics" "$work/artifacts" "$work/logs"
  : > "$work/alertmanager.yml"
  # 空 override（dev 模式下 worker 不读它也必须能启动）
  : > "$work/config_override.yaml"

  # 本地 overlay：宿主路径全部指到临时目录（volumes 覆写为整列表替换）。
  cat > "$work/overlay.yml" <<EOF
services:
  migrate:
    volumes:
      - $work/artifacts:/app/evidence
      - $work/metrics:/var/lib/omnicraft/metrics
  worker:
    volumes:
      - $work/config_override.yaml:/app/config_override.yaml:ro
EOF

  # env_file（backend/worker 的容器内变量）落在 base compose 同目录；本地临时生成。
  cat > "$DOTENV" <<'EOF'
SERVER_MODE=local
EOF
  # 先拆栈再清文件：down 需要env file 完成插值。
  trap 'docker compose --project-directory "$ROOT" -p omnicraft-f3-verify -f "$COMPOSE_BASE" -f "$WORK_DIR/overlay.yml" --env-file "$ENV_FILE" down -v >/dev/null 2>&1 || true; rm -f "$ENV_FILE" "$DOTENV"; rm -rf "$WORK_DIR"' EXIT

  echo ">> 起最小栈（postgres redis pgbouncer migrate worker）…"
  docker compose --project-directory "$ROOT" -p omnicraft-f3-verify -f "$COMPOSE_BASE" -f "$work/overlay.yml" \
    --env-file "$ENV_FILE" up -d --build postgres redis pgbouncer migrate worker >/dev/null

  echo ">> 等 worker healthy…"
  local i=0
  until [ "$(docker inspect --format '{{.State.Health.Status}}' "omnicraft-f3-verify-worker-1" 2>/dev/null || true)" = "healthy" ]; do
    i=$((i + 1)); [ $i -gt 60 ] && { docker logs omnicraft-f3-verify-worker-1 2>&1 | tail -20; fail "worker 未在 180s 内 healthy"; }
    sleep 3
  done
  pass "worker 容器 healthy"

  # 造数：一个真实 user（notifications.user_id FK）+ 一条通知事件。
  echo ">> 投递通知事件…"
  docker exec omnicraft-f3-verify-postgres-1 psql -U omnicraft -d omnicraft \
    -c "INSERT INTO users (email, password_hash, username) VALUES ('f3-verify@local.test','x','f3_verify_user') ON CONFLICT DO NOTHING" >/dev/null
  local uid
  uid="$(docker exec omnicraft-f3-verify-postgres-1 psql -U omnicraft -d omnicraft -tAc "SELECT id FROM users WHERE username='f3_verify_user'")"
  # 幂等口径：保留卷里可能已有上一轮的行，本轮落库 = count 相比投递前增加。
  # before 必须先于 XADD 取——worker 亚秒级消费，投递后再取快照会把本轮行算进基线。
  local before count lag ok=0
  before="$(docker exec omnicraft-f3-verify-postgres-1 psql -U omnicraft -d omnicraft -tAc "SELECT count(*) FROM notifications WHERE user_id=${uid} AND title='F3 verify'" | tr -d '[:space:]')"
  docker exec omnicraft-f3-verify-redis-1 redis-cli -a local-verify-redis-pass --no-auth-warning \
    XADD omnicraft:notification.create '*' \
    payload "{\"user_id\":${uid},\"channel\":\"system\",\"type\":\"system\",\"title\":\"F3 verify\",\"body\":\"compose worker definition\",\"target_type\":\"\",\"target_id\":0,\"sender_id\":0}" \
    timestamp "$(date +%s000)" >/dev/null

  echo ">> 等消费落库 + lag 回 0…"
  for i in $(seq 1 30); do
    count="$(docker exec omnicraft-f3-verify-postgres-1 psql -U omnicraft -d omnicraft -tAc "SELECT count(*) FROM notifications WHERE user_id=${uid} AND title='F3 verify'" | tr -d '[:space:]')"
    lag="$(docker exec omnicraft-f3-verify-redis-1 redis-cli -a local-verify-redis-pass --no-auth-warning xinfo groups omnicraft:notification.create | grep -A1 '^lag' | tail -1 | tr -d '[:space:]')"
    if [ "${count:-0}" -gt "${before:-0}" ] && [ "$lag" = "0" ]; then ok=1; break; fi
    sleep 2
  done
  [ $ok -eq 1 ] || { docker logs omnicraft-f3-verify-worker-1 2>&1 | tail -20; fail "通知未落库或 lag 未回 0（before=$before count=${count:-?} lag=${lag:-?}）"; }
  pass "通知事件消费落库（user_id=${uid}）且 xinfo lag=0"

  echo "local: ALL PASS"
}

case "$MODE" in
  --check) check_static ;;
  --local) check_local ;;
  *) echo "usage: $0 --check | --local" >&2; exit 2 ;;
esac
