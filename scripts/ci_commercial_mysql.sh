#!/usr/bin/env bash
# Workflow-owned durable database only; never touches the developer database.
set -euo pipefail
[[ "${GITHUB_ACTIONS:-}" == true ]] || { echo 'CI_OWNED_DATABASE_ONLY' >&2; exit 2; }
operation="${1:?}"; code="${2:?}"
[[ "$code" =~ ^ce0[4-7]$ && "${GITHUB_RUN_ID:-}" =~ ^[0-9]+$ && "${GITHUB_RUN_ATTEMPT:-}" =~ ^[0-9]+$ ]] || exit 2
out="${RUNNER_TEMP:?}/$code"
container="batch-a-$code-$GITHUB_RUN_ID-$GITHUB_RUN_ATTEMPT"
label="$code-$GITHUB_RUN_ID-$GITHUB_RUN_ATTEMPT"
mkdir -p "$out"

assert_owner() {
  [[ "$(docker inspect -f '{{ index .Config.Labels "ci.batch-a" }}' "$container")" == "$label" ]]
}

case "$operation" in
start)
  # Refuse collisions rather than removing a container whose ownership is unknown.
  if docker inspect "$container" >/dev/null 2>&1; then echo 'CONTAINER_ALREADY_EXISTS' >&2; exit 2; fi
  date +%s > "$out/mysql.started"
  echo "MYSQL_CONTAINER_ID=$container" >> "${GITHUB_ENV:?}"
  nohup setsid bash "$0" prepare "$code" > "$out/mysql-launch.log" 2>&1 < /dev/null &
  echo "$!" > "$out/mysql-launch.pid"
  echo "CE_MYSQL_LAUNCH_STARTED=$code"
  ;;
prepare)
  trap 'rc=$?; echo "$rc" > "$out/mysql-launch.exit"' EXIT
  docker run --name "$container" --label "ci.batch-a=$label" \
    -e MYSQL_ROOT_PASSWORD=root -e "MYSQL_DATABASE=biz_$code" \
    -p 127.0.0.1:3306:3306 -d mysql:8.4 > "$out/mysql-container.id"
  assert_owner
  # Authenticate on TCP and select the expected DB, not only mysqladmin's liveness.
  for attempt in $(seq 1 90); do
    if docker exec "$container" mysql --protocol=TCP -h 127.0.0.1 -uroot -proot \
        "biz_$code" -Nse 'SELECT 1' >/dev/null 2>&1; then
      date +%s > "$out/mysql.ready"
      docker inspect -f '{{.Image}} {{.State.StartedAt}}' "$container" > "$out/mysql-identity.txt"
      echo "CE_MYSQL_READY=$code elapsed=$(( $(cat "$out/mysql.ready") - $(cat "$out/mysql.started") ))s"
      exit 0
    fi
    sleep 1
  done
  docker logs "$container" >&2 || true
  exit 1
  ;;
wait)
  # Total bootstrap bound includes image download; no unbounded silent wait.
  for attempt in $(seq 1 150); do
    if [[ -f "$out/mysql-launch.exit" ]]; then
      [[ "$(cat "$out/mysql-launch.exit")" == 0 && -f "$out/mysql.ready" ]] || { cat "$out/mysql-launch.log" >&2; exit 1; }
      assert_owner
      echo "CE_MYSQL_READY_SECONDS=$(( $(cat "$out/mysql.ready") - $(cat "$out/mysql.started") ))"
      exit 0
    fi
    elapsed="$(( $(date +%s) - $(cat "$out/mysql.started") ))"
    (( elapsed < 150 )) || { cat "$out/mysql-launch.log" >&2; echo 'MYSQL_BOOTSTRAP_TIMEOUT' >&2; exit 1; }
    if (( attempt % 10 == 1 )); then echo "CE_MYSQL_WAITING=$code elapsed=${elapsed}s"; fi
    sleep 1
  done
  exit 1
  ;;
stop)
  if [[ -f "$out/mysql-launch.pid" && ! -f "$out/mysql-launch.exit" ]]; then
    kill -- -"$(cat "$out/mysql-launch.pid")" 2>/dev/null || true
    sleep 1
  fi
  if docker inspect "$container" >/dev/null 2>&1; then
    assert_owner
    docker logs "$container" > "$out/mysql-server.log" 2>&1 || true
    docker rm -fv "$container" >/dev/null
  fi
  ;;
*) exit 2 ;;
esac
