#!/bin/sh
set -eu

cd "$(dirname "$0")/.."

counter_packets() {
    counter_output=$(docker compose exec -T gateway nft list counter inet lab_filter "$1")
    printf '%s\n' "$counter_output" | awk '
        { for (i = 1; i < NF; i++) if ($i == "packets") { print $(i + 1); found = 1; exit } }
        END { if (!found) exit 1 }
    '
}

check_blocked() {
    client=$1
    counter=$2
    before=$(counter_packets "$counter")
    if docker compose exec -T "$client" wget -T 3 -t 1 -qO- http://172.28.20.10; then
        printf 'FAIL: %s reached Internal Site\n' "$client" >&2
        return 1
    fi
    after=$(counter_packets "$counter")
    if [ "$after" -le "$before" ]; then
        printf 'FAIL: %s request failed without increasing Firewall counter %s\n' "$client" "$counter" >&2
        return 1
    fi
    printf 'PASS: %s blocked; %s packets %s -> %s\n' "$client" "$counter" "$before" "$after"
}

# Positive controls prevent an unavailable site or disconnected client from
# being mistaken for successful network isolation.
docker compose exec -T internal-site wget -T 3 -t 1 -qO- http://172.28.20.10
printf '\nPASS: Internal Site responds inside Corporate\n'
docker compose exec -T gateway wget -T 3 -t 1 -qO- http://172.28.20.10
printf '\nPASS: Gateway reaches Internal Site\n'
docker compose exec -T guest-client ping -c 1 -W 3 172.28.10.254
docker compose exec -T iot-client ping -c 1 -W 3 172.28.30.254

check_blocked guest-client guest_to_corporate
check_blocked iot-client iot_to_corporate
docker compose exec -T gateway nft list table inet lab_filter
