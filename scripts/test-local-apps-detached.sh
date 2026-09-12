#!/usr/bin/env bash
set -euo pipefail

pid=$(python3 - <<'PY'
import subprocess

child = subprocess.Popen(
    ["sleep", "5"],
    stdin=subprocess.DEVNULL,
    stdout=subprocess.DEVNULL,
    stderr=subprocess.DEVNULL,
    start_new_session=True,
)
print(child.pid)
PY
)
kill -0 "$pid"
kill -TERM "$pid"
while kill -0 "$pid" 2>/dev/null; do sleep 0.05; done
