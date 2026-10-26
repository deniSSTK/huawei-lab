#!/bin/sh
set -eu

cd "$(dirname "$0")/.."
export LAB_EVIDENCE_WORKSPACE="$(pwd -P)"
export LAB_EVIDENCE_HOME="$HOME"
export LAB_EVIDENCE_HOST="$(uname -n)"

sanitize_output() {
    # Literal replacement keeps paths/hostnames from being treated as regexes.
    awk '
        function replace_literal(value, old, replacement, position, prefix) {
            if (old == "") return value
            prefix = ""
            while ((position = index(value, old)) > 0) {
                prefix = prefix substr(value, 1, position - 1) replacement
                value = substr(value, position + length(old))
            }
            return prefix value
        }
        {
            line = replace_literal($0, ENVIRON["LAB_EVIDENCE_WORKSPACE"], "/workspace/huawei-lab")
            line = replace_literal(line, ENVIRON["LAB_EVIDENCE_HOME"], "/home/lab-user")
            print replace_literal(line, ENVIRON["LAB_EVIDENCE_HOST"], "lab-host")
        }
    '
}

evidence_dir="evidence/sprint-2/$(TZ=Europe/Kiev date +%Y-%m-%d_%H-%M-%S)-isolation"
mkdir -p "$evidence_dir"

# Save both successful and failed command output, with its actual exit status.
record() {
    log_file=$1
    shift
    printf '\n$ %s\n' "$*" | sanitize_output >> "$log_file"
    raw_output=$(mktemp)
    if "$@" > "$raw_output" 2>&1; then
        result=0
    else
        result=$?
    fi
    sanitize_output < "$raw_output" >> "$log_file"
    rm -f "$raw_output"
    printf '\nexit_status=%s\n' "$result" >> "$log_file"
    if [ "$result" -ne 0 ]; then
        cat "$log_file"
    fi
    return "$result"
}

metadata="$evidence_dir/environment.log"
record "$metadata" env TZ=Europe/Kiev date '+%Y-%m-%dT%H:%M:%S%z'
record "$metadata" uname -a
if command -v sw_vers >/dev/null 2>&1; then
    record "$metadata" sw_vers
fi
record "$metadata" git rev-parse HEAD
record "$metadata" git status --short
record "$metadata" shasum -a 256 docker-compose.yml gateway/Dockerfile gateway/entrypoint.sh gateway/nftables.conf scripts/check-isolation.sh
record "$metadata" docker version
record "$metadata" docker compose version
record "$metadata" docker compose config

run_checks() {
    stage=$1
    log_file="$evidence_dir/$stage.log"
    record "$log_file" docker compose ps
    # Container IDs identify the processes tested before/after recreation.
    record "$log_file" docker compose ps -q
    record "$log_file" docker compose exec -T gateway sysctl net.ipv4.ip_forward
    record "$log_file" docker compose exec -T guest-client ip route
    record "$log_file" docker compose exec -T iot-client ip route
    record "$log_file" docker compose exec -T internal-site ip route
    record "$log_file" sh scripts/check-isolation.sh
    printf 'PASS: %s; log: %s\n' "$stage" "$log_file"
}

record "$evidence_dir/01-initial.log" docker compose config --quiet
record "$evidence_dir/01-initial.log" docker compose up -d --build --wait --wait-timeout 60
run_checks 01-initial

record "$evidence_dir/02-restart.log" docker compose restart gateway
record "$evidence_dir/02-restart.log" docker compose up -d --wait --wait-timeout 60
run_checks 02-restart

record "$evidence_dir/03-recreate.log" docker compose up -d --force-recreate --wait --wait-timeout 60
run_checks 03-recreate

printf 'Isolation evidence saved to %s\n' "$evidence_dir"
printf 'Guest internet was not tested; it is a separate work item.\n'
