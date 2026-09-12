#!/usr/bin/env python3
"""Guard the one-off CE02 archived-fixture catalog repair on the shared DB."""
import fcntl
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[1]
CONTAINER = os.environ["EVOLUTION_MYSQL_CONTAINER"]
PASSWORD = os.environ["EVOLUTION_MYSQL_PASSWORD"]
DSN = os.environ["YUNKA_BIZ_MYSQL_DSN"]
if CONTAINER != "biz-evolution-mysql-20260912":
    raise SystemExit("repair must use biz-evolution-mysql-20260912")
if not re.search(r"@tcp\(127\.0\.0\.1:13316\)/biz_evolution(?:\?|$)", DSN):
    raise SystemExit("repair must use tcp(127.0.0.1:13316)/biz_evolution")
common = Path(subprocess.check_output(["git", "rev-parse", "--git-common-dir"], cwd=ROOT, text=True).strip())
if not common.is_absolute():
    common = (ROOT / common).resolve()
backups = common / "local-db-backups"
backups.mkdir(exist_ok=True)
lock = (backups / "verification.lock").open("a")
try:
    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
except BlockingIOError:
    raise SystemExit("another shared-database verification is running")
marker = backups / "recovery-required.json"
if marker.exists():
    raise SystemExit("recovery marker exists; restore the preserved backup before retrying")
env = dict(os.environ, MYSQL_PWD=PASSWORD)
admin = ["docker", "exec", "-e", "MYSQL_PWD", CONTAINER, "mysql", "-uroot", "-N", "-B", "biz_evolution"]
ports = json.loads(subprocess.check_output(["docker", "inspect", "--format", "{{json .NetworkSettings.Ports}}", CONTAINER], text=True))
if not any(row["HostIp"] == "127.0.0.1" and row["HostPort"] == "13316" for row in ports.get("3306/tcp", []) or []):
    raise SystemExit("container port mapping does not match the repair DSN")
clients = int(subprocess.check_output(admin + ["-e", "SELECT COUNT(*) FROM information_schema.PROCESSLIST WHERE DB=DATABASE() AND ID<>CONNECTION_ID()"], env=env, text=True).strip())
if clients:
    raise SystemExit("stop application/database clients before repair")
backup = backups / ("database-before-ce02-archived-fixture-repair-" + str(time.time_ns()) + ".sql")
with backup.open("xb") as stream:
    backup.chmod(0o600)
    subprocess.run(["docker", "exec", "-e", "MYSQL_PWD", CONTAINER, "mysqldump", "-uroot", "--single-transaction", "--no-tablespaces", "--set-gtid-purged=OFF", "biz_evolution"], env=env, stdout=stream, check=True)
with marker.open("x") as stream:
    marker.chmod(0o600)
    json.dump({"container": CONTAINER, "database": "biz_evolution", "backup": str(backup), "operation": "ce02-archived-fixture-repair"}, stream)
child_env = dict(os.environ, CE02_REPAIR_RECOVERY_MARKER=str(marker), CE02_REPAIR_BACKUP=str(backup))
try:
    subprocess.run(["go", "run", "./cmd/biz-local-ce02-repair"], cwd=ROOT, env=child_env, check=True)
except BaseException:
    # The marker intentionally remains: operators restore the unique backup
    # under the existing recovery policy before any retry.
    raise
marker.unlink()
print("CE02 archived-fixture repair complete; recovery marker cleared; backup=" + str(backup))
