#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STATE="$ROOT/.local/backend"
BIN="$STATE/mahjong-local"
PID_FILE="$STATE/server.pid"
LOG_FILE="$STATE/server.log"
URL="http://127.0.0.1:8081"

managed_pid() {
  [[ -f "$PID_FILE" ]] || return 1
  local pid command
  pid="$(cat "$PID_FILE")"
  [[ "$pid" =~ ^[0-9]+$ ]] || return 1
  kill -0 "$pid" 2>/dev/null || return 1
  command="$(ps -p "$pid" -o command=)"
  [[ "$command" == "$BIN" ]] || return 1
  printf '%s\n' "$pid"
}

status() {
  local pid
  if pid="$(managed_pid)"; then
    printf '后端运行中，PID %s\n%s\n' "$pid" "$URL"
    curl --fail --silent --show-error --max-time 2 "$URL/healthz"
    printf '\n'
  else
    printf '脚本管理的后端未运行\n'
    return 1
  fi
}

start() {
  if managed_pid >/dev/null; then status; return; fi
  if lsof -nP -iTCP:8081 -sTCP:LISTEN >/dev/null 2>&1; then
    printf '8081 端口已被其他进程占用，请先检查；脚本不会停止该进程。\n' >&2
    lsof -nP -iTCP:8081 -sTCP:LISTEN >&2
    return 1
  fi
  mkdir -p "$STATE"
  printf '编译联机后端…\n'
  (cd "$ROOT/backend" && go build -o "$BIN" ./cmd/local)
  (
    cd "$ROOT/backend"
    # Create a separate session so the launching terminal's process group can exit.
    ADDR=127.0.0.1:8081 nohup perl -MPOSIX -e 'POSIX::setsid() != -1 or die "setsid: $!"; exec @ARGV or die "exec: $!"' "$BIN" >>"$LOG_FILE" 2>&1 </dev/null &
    echo $! >"$PID_FILE"
  )
  local attempt
  for attempt in {1..50}; do
    if managed_pid >/dev/null && curl --fail --silent --max-time 1 "$URL/healthz" >/dev/null; then
      status
      printf '日志：%s\n' "$LOG_FILE"
      return
    fi
    sleep 0.2
  done
  printf '启动失败或超时，请查看日志：%s\n' "$LOG_FILE" >&2
  tail -n 30 "$LOG_FILE" >&2
  return 1
}

stop() {
  local pid attempt
  if ! pid="$(managed_pid)"; then
    printf '脚本管理的后端未运行，无需停止。\n'
    return
  fi
  kill -TERM "$pid"
  for attempt in {1..30}; do
    if ! managed_pid >/dev/null; then
      rm -f "$PID_FILE"
      printf '后端已停止。内存中的房间随服务结束清空。\n'
      return
    fi
    sleep 0.1
  done
  printf '进程尚未退出，请检查 PID %s；未强制结束。\n' "$pid" >&2
  return 1
}

case "${1:-start}" in
  start) start ;;
  stop) stop ;;
  restart) stop; start ;;
  status) status ;;
  logs) if [[ -f "$LOG_FILE" ]]; then tail -n 80 "$LOG_FILE"; else printf '尚无日志。\n'; fi ;;
  *) printf '用法：%s {start|stop|restart|status|logs}\n' "$0" >&2; exit 2 ;;
esac
