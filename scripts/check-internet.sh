#!/bin/sh
set -eu

cd "$(dirname "$0")/.."

counter_packets() {
    counter_output=$(docker compose exec -T gateway nft list counter inet lab_filter guest_to_internet)
    printf '%s\n' "$counter_output" | awk '
        { for (i = 1; i < NF; i++) if ($i == "packets") { print $(i + 1); found = 1; exit } }
        END { if (!found) exit 1 }
    '
}

guest_routes=$(docker compose exec -T guest-client ip -4 route)
printf '%s\n' "$guest_routes"
printf '%s\n' "$guest_routes" | awk '
    $1 == "default" { count++; if ($2 != "via" || $3 != "172.28.10.254") wrong = 1 }
    END { if (count != 1 || wrong) exit 1 }
'
printf 'PASS: Guest default route uses the lab Gateway\n'

uplink_routes=$(docker compose exec -T gateway ip -4 route)
printf '%s\n' "$uplink_routes"
printf '%s\n' "$uplink_routes" | awk '
    $1 == "default" { count++; if ($2 != "via" || $3 != "172.28.40.1") wrong = 1 }
    END { if (count != 1 || wrong) exit 1 }
'
printf 'PASS: Gateway default route uses the uplink\n'

for url in http://example.com/ https://example.com/; do
    before=$(counter_packets)
    # Bound the entire operation, verify HTTPS certificates, force IPv4, and
    # disable proxies so the request uses the client's own default route.
    if ! body=$(docker compose exec -T guest-client curl -4 --noproxy '*' --connect-timeout 5 --max-time 20 --fail --silent --show-error "$url"); then
        printf 'FAIL: Guest request to %s failed\n' "$url" >&2
        exit 1
    fi
    printf '%s\n' "$body" | grep -q 'Example Domain'
    after=$(counter_packets)
    if [ "$after" -le "$before" ]; then
        printf 'FAIL: external response without Gateway web counter increase\n' >&2
        exit 1
    fi
    printf 'PASS: Guest fetched %s (Example Domain); guest_to_internet packets %s -> %s\n' "$url" "$before" "$after"
done

docker compose exec -T gateway nft list counter inet lab_filter guest_to_internet
docker compose exec -T gateway nft list table ip lab_nat
