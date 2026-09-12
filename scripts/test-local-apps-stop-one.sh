#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
program=$(awk '/^case / { exit } { print }' "$root/scripts/local-apps.sh")
program+=$'\nstate_dir=$(mktemp -d)\n'
program+='trap "rmdir \"$state_dir\"" EXIT'
program+=$'\nstop_one web /not-an-owned-process 14183\n'

# The empty state directory makes stop_one return before process inspection or
# kill. The dispatcher and startup path are never loaded.
bash -u -c "$program"
