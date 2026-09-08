#!/usr/bin/env python3
"""Exercise the actual B12.7 shell readiness gate with controlled evidence timing."""
from pathlib import Path
import os
import subprocess
import tempfile
import shutil


def readiness_gate() -> str:
    workflow = Path(__file__).resolve().parents[1] / ".github/workflows/b12-7-runtime-qualification.yml"
    lines = workflow.read_text().splitlines()
    start = next(i for i, line in enumerate(lines) if line.strip() == "ready=0")
    end = next(i for i in range(start, len(lines)) if lines[i].strip() == 'grep -Fq \'DEV READY\' "$RUNNER_TEMP/b12-7-dev.stderr"')
    gate = "\n".join(line.removeprefix("          ") for line in lines[start:end + 1])
    if gate.count("seq 1 240") != 1:
        raise AssertionError("B12.7 readiness gate changed; update the bounded regression explicitly")
    return gate.replace("seq 1 240", "seq 1 4")


def exercise(name: str, gate: str, ready: bool, delayed_log: bool, log_present: bool, expected: int) -> None:
    with tempfile.TemporaryDirectory(prefix="biz-readiness-") as directory:
        root = Path(directory)
        (root / ".yunka").mkdir()
        (root / ".yunka/dev-runtime.json").write_text(
            '{"state":"running","processes":[{"state":"ready","ready":true}]}'
            if ready else '{"state":"running","processes":[{"state":"starting","ready":false}]}')
        log = root / "b12-7-dev.stderr"
        log.write_text("DEV READY application=biz\n" if log_present else "")
        (root / "b12-7-dev.stdout").write_text("")
        env = {**os.environ, "RUNNER_TEMP": str(root)}
        if delayed_log:
            # Deterministic one-observation lag: real grep misses once, then
            # readiness is published before the next poll. No sleep-based race.
            real_grep = shutil.which("grep")
            if not real_grep:
                raise AssertionError("grep is required")
            bindir = root / "bin"
            bindir.mkdir()
            shim = bindir / "grep"
            shim.write_text("#!/usr/bin/env python3\nimport subprocess,sys,pathlib\n"
                + f"result=subprocess.run([{real_grep!r}]+sys.argv[1:])\n"
                + f"log=pathlib.Path({str(log)!r})\n"
                + "if result.returncode == 1 and not log.read_text():\n"
                + "    log.write_text('DEV READY application=biz\\n')\n"
                + "sys.exit(result.returncode)\n")
            shim.chmod(0o700)
            env["PATH"] = str(bindir) + os.pathsep + env.get("PATH", "")
        result = subprocess.run(
            ["bash", "-c", 'set -euo pipefail\ndev_pid=$$\n' + gate],
            cwd=root, env=env, capture_output=True, text=True, timeout=5, check=False)
        if result.returncode != expected:
            raise AssertionError(f"{name}: exit={result.returncode}, expected={expected}; {result.stderr}")
        print(f"B12.7 readiness gate {name}: PASS")


if __name__ == "__main__":
    gate = readiness_gate()
    exercise("delayed_log_after_ready_state", gate, True, True, False, 0)
    exercise("missing_log_rejected", gate, True, False, False, 1)
    exercise("log_without_ready_state_rejected", gate, False, False, True, 1)
