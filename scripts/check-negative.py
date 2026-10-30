#!/usr/bin/env python3
"""Lab 16: record negative checks without changing the frozen network files.

Starts a temporary second nginx listener on Corporate TCP/8080, then stops it.
Restarts the gateway and recreates the lab containers for recovery checks.
Run from the repository root with Docker available. Evidence is never overwritten.
"""
import datetime
import hashlib
import json
from pathlib import Path
import re
import shlex
import subprocess
import sys
from zoneinfo import ZoneInfo

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "evidence/testing-2"
OUT.mkdir(parents=True, exist_ok=True)
if (OUT / "execution.log").exists():
    sys.exit("Evidence already exists; move it to an archive before a new run.")


def now():
    return datetime.datetime.now(ZoneInfo("Europe/Kiev")).isoformat(timespec="seconds")


def append(filename, text):
    for name in (filename, "execution.log"):
        with (OUT / name).open("a") as stream:
            stream.write(text)


def run(filename, args, expected=(0,)):
    header = f"\n[{now()}]\n$ {shlex.join(args)}\n"
    result = subprocess.run(args, cwd=ROOT, text=True, stdout=subprocess.PIPE,
                            stderr=subprocess.STDOUT, timeout=180)
    append(filename, header + result.stdout + f"\nexit_status={result.returncode}\n")
    if result.returncode not in expected:
        raise RuntimeError(f"Unexpected exit {result.returncode}: {shlex.join(args)}")
    return result.stdout


def dc(filename, *args, expected=(0,)):
    return run(filename, ["docker", "compose", *args], expected)


def counter(filename, name):
    output = dc(filename, "exec", "-T", "gateway", "nft", "list", "counter",
                "inet", "lab_filter", name)
    return int(re.search(r"packets (\d+)", output).group(1))


def result(filename, text):
    append(filename, f"\n{text}\n")
    print(text, flush=True)


start = now()
expected = {
    "baseline": "v1.0.0 / e601531989e28694bbddf5b43fceb88154d55daa",
    "NT-01": "Gateway reaches live Corporate TCP/8080; Guest times out; guest_to_corporate increases.",
    "NT-02": "Gateway reaches live Corporate TCP/8080; IoT times out; iot_to_corporate increases.",
    "NT-03": "After gateway restart and container recreation, Guest/IoT isolation and Guest HTTP/HTTPS still pass after readiness.",
    "NT-04": "Invalid credentials, wrong role and expired session must not grant access. BLOCKED if portal is absent.",
    "scope": "TCP/8080 is the selected alternative port; UDP and downtime continuity are not tested. Temporary listener is a test fixture, not a baseline change.",
    "recorded_before_execution": start,
}
(OUT / "expected.json").write_text(json.dumps(expected, indent=2) + "\n")
statuses = {}
try:
    meta = "environment.log"
    head = run(meta, ["git", "rev-parse", "HEAD"]).strip()
    tag = run(meta, ["git", "rev-parse", "v1.0.0^{commit}"]).strip()
    if head != tag or head != "e601531989e28694bbddf5b43fceb88154d55daa":
        raise RuntimeError("Working checkout is not the expected freeze commit")
    run(meta, ["git", "diff", "--exit-code", "HEAD", "--", "docker-compose.yml", "gateway", "client"])
    run(meta, ["git", "status", "--short"])
    run(meta, ["docker", "version"])
    run(meta, ["docker", "compose", "version"])
    dc(meta, "config")
    dc(meta, "ps")
    hashes = {str(p.relative_to(ROOT)): hashlib.sha256(p.read_bytes()).hexdigest()
              for p in [ROOT / "docker-compose.yml", *sorted((ROOT / "gateway").glob("*")),
                        ROOT / "client/Dockerfile", ROOT / "scripts/check-negative.py",
                        ROOT / "scripts/check-isolation.sh", ROOT / "scripts/check-internet.sh"] if p.is_file()}
    (OUT / "sha256.json").write_text(json.dumps(hashes, indent=2) + "\n")
    config = ('pid /tmp/lab16-nginx.pid; error_log /tmp/lab16-error.log; '
              'events {} http { access_log /tmp/lab16-access.log; server { '
              'listen 8080; location / { return 200 "LAB16_TCP_8080_OK\\n"; } } }')
    setup = "printf '%s\\n' " + shlex.quote(config) + " > /tmp/lab16-nginx.conf && nginx -c /tmp/lab16-nginx.conf"
    dc(meta, "exec", "-T", "internal-site", "sh", "-c", setup)
    try:
        for scenario, client, name in [("NT-01", "guest-client", "guest_to_corporate"),
                                       ("NT-02", "iot-client", "iot_to_corporate")]:
            log = scenario + ".log"
            try:
                control = dc(log, "exec", "-T", "gateway", "wget", "-T", "5", "-t", "1", "-qO-", "http://172.28.20.10:8080/")
                if "LAB16_TCP_8080_OK" not in control:
                    raise RuntimeError("Positive control response missing")
                dc(log, "exec", "-T", client, "ip", "route")
                gateway_ip = "172.28.10.254" if client == "guest-client" else "172.28.30.254"
                dc(log, "exec", "-T", client, "ping", "-c", "1", "-W", "3", gateway_ip)
                before = counter(log, name)
                if client == "guest-client":
                    response = dc(log, "exec", "-T", client, "curl", "-4", "--noproxy", "*", "--connect-timeout", "5", "--max-time", "5", "--silent", "--show-error", "http://172.28.20.10:8080/", expected=(28,))
                    if "timed out" not in response.lower():
                        raise RuntimeError("Expected connection timeout")
                else:
                    response = dc(log, "exec", "-T", client, "wget", "-Y", "off", "-O-", "-T", "5", "-t", "1", "http://172.28.20.10:8080/", expected=(1,))
                    if "timed out" not in response.lower():
                        raise RuntimeError("Expected connection timeout")
                after = counter(log, name)
                if after <= before:
                    raise RuntimeError("Firewall counter did not increase")
                statuses[scenario] = {"status": "PASS", "counter": name, "before": before, "after": after}
                result(log, f"PASS: {scenario}; {client} blocked on TCP/8080; {name} packets {before} -> {after}")
            except Exception as exc:
                statuses[scenario] = {"status": "FAIL", "error": str(exc)}
                result(log, f"FAIL: {scenario}; {exc}")
    finally:
        dc(meta, "exec", "-T", "internal-site", "nginx", "-s", "quit", "-c", "/tmp/lab16-nginx.conf")
        result(meta, "Temporary TCP/8080 listener stopped; original nginx remains running.")

    log = "NT-03.log"
    try:
        for stage in ("initial", "restart", "recreate"):
            result(log, f"STAGE: {stage}")
            if stage == "restart":
                dc(log, "restart", "gateway")
                dc(log, "up", "-d", "--wait", "--wait-timeout", "60")
            elif stage == "recreate":
                dc(log, "up", "-d", "--force-recreate", "--wait", "--wait-timeout", "60")
            dc(log, "ps")
            dc(log, "ps", "-q")
            run(log, ["sh", "scripts/check-isolation.sh"])
            run(log, ["sh", "scripts/check-internet.sh"])
            result(log, f"PASS: NT-03 {stage}; isolation and Guest internet verified")
        statuses["NT-03"] = {"status": "PASS", "stages": ["initial", "restart", "recreate"]}
    except Exception as exc:
        statuses["NT-03"] = {"status": "FAIL", "error": str(exc)}
        result(log, f"FAIL: NT-03; {exc}")

    services = dc("NT-04.log", "config", "--services")
    statuses["NT-04"] = {"status": "BLOCKED", "reason": "Auth Portal, roles and sessions are absent from frozen baseline", "services": services.splitlines()}
    result("NT-04.log", "BLOCKED: NT-04; no Auth Portal service. Negative login/role/session actions were not executed. Issue D-01; handoff to Lab 17.")
    run(meta, ["git", "diff", "--exit-code", "HEAD", "--", "docker-compose.yml", "gateway", "client"])
finally:
    (OUT / "results.json").write_text(json.dumps({"started": start, "finished": now(), "scenarios": statuses}, indent=2) + "\n")

sys.exit(1 if len(statuses) != 4 or any(s["status"] == "FAIL" for s in statuses.values()) else 0)
