#!/usr/bin/env bash
set -euo pipefail
container="${EVOLUTION_MYSQL_CONTAINER:-biz-evolution-mysql-20260912}"
case "${1:-status}" in
  start) docker start "$container" ;;
  status) docker ps -a --filter "name=^/${container}$" --format '{{.Names}} {{.Status}} {{.Ports}}' ;;
  *) echo 'Usage: scripts/local-mysql.sh [start|status]' >&2; exit 1 ;;
esac
