# Sprint 2: Firewall Foundation

[Українська версія](sprint-2-ua.md)

## Goal and scope

Extend the Sprint 1 topology with repeatable Guest-to-Corporate and IoT-to-Corporate isolation and Guest web access through the gateway. This implements the network foundation for R-01, R-02, R-07 and D-01 from the supplied templates. The final Lab 13 Sprint 2 Record will be prepared after the baseline commit and screenshots; the Implementation Freeze decision belongs to Lab 14.

## Implementation

- `gateway/Dockerfile` installs nftables at image build time.
- `client/Dockerfile` adds curl and CA certificates to Guest for verified HTTPS checks.
- `gateway/nftables.conf` owns `inet lab_filter`; `gateway/nat.conf` owns `ip lab_nat`. The entrypoint loads both tables atomically without flushing unrelated tables or accumulating duplicate rules.
- Only the gateway joins uplink `172.28.40.0/24` at `172.28.40.254`. Its default route uses Docker's `172.28.40.1`, selected by `gw_priority: 1` (Compose 2.33.1+). The entrypoint resolves the interface by its IP, not an assumed `eth0` name.
- Docker Compose enables IPv4 forwarding. The entrypoint loads the rules before the gateway becomes healthy; configuration failure stops the gateway. Other services wait for its healthcheck on Compose startup. This startup ordering is not an atomic fail-closed guarantee during gateway recreation while existing clients are running; validate the policy after recreation before resuming tests.
- Guest (`172.28.10.0/24`) and IoT (`172.28.30.0/24`) packets to Corporate (`172.28.20.0/24`) are dropped before the established/related rule. Named counters identify the two restrictions.
- Guest may forward TCP 80/443 and UDP/TCP 53 through the uplink. `guest_to_internet` counts web traffic; `guest_dns` counts forwarded DNS. Guest source addresses are masqueraded on the uplink, with return packets allowed by established/related. Other new traffic, including IoT internet, is dropped. Gateway-local input/output is unchanged, so clients can ping their gateway and the gateway can check Nginx directly.
- Docker's embedded DNS may resolve domains separately. Internet checks verify real HTTP/HTTPS responses from `example.com` and an increasing gateway web counter, with proxies disabled, IPv4 forced, certificate verification, and bounded timeouts.

The rules apply inside the gateway container, not to the host Firewall. This remains a trusted-client lab: client containers retain `NET_ADMIN` for route setup. It does not protect against a participant deliberately changing routes or spoofing an IP.

## Checks to run

```bash
docker compose config --quiet
docker compose up -d --build --wait
docker compose ps
sh scripts/check-isolation.sh
sh scripts/check-internet.sh
docker compose restart gateway
docker compose up -d --wait
sh scripts/check-isolation.sh
sh scripts/check-internet.sh
docker compose up -d --force-recreate --wait
sh scripts/check-isolation.sh
sh scripts/check-internet.sh
```

The script fails unless the internal server and paths to the gateway work, both internal HTTP requests fail, and each named Firewall counter increases. A timeout alone is not accepted as evidence of isolation. Stop other client traffic during the check so counters identify the test requests.

For terminal screenshots, the corresponding manual commands are:

```bash
docker compose exec gateway nft list table inet lab_filter
docker compose exec internal-site wget -T 3 -t 1 -qO- http://172.28.20.10
docker compose exec gateway wget -T 3 -t 1 -qO- http://172.28.20.10
docker compose exec guest-client ping -c 1 172.28.10.254
docker compose exec iot-client ping -c 1 172.28.30.254
docker compose exec guest-client wget -T 3 -t 1 -qO- http://172.28.20.10
docker compose exec iot-client wget -T 3 -t 1 -qO- http://172.28.20.10
docker compose exec gateway nft list table inet lab_filter
```

The two client HTTP commands should fail; the server and gateway HTTP commands should succeed. Compare Firewall counters before and after the client requests.

## Validation state

The integrated checks passed on 27 October 2026 after startup, gateway restart and recreation of all containers: Nginx and the gateways respond, Guest/IoT requests to Corporate are blocked with increasing drop counters, and Guest receives HTTP/HTTPS responses through the gateway. See the single [results report with environment and logs](../evidence/sprint-2/results.md). Repeat collection with `sh scripts/collect-isolation-evidence.sh`.

The [portal description and planned role matrix](../docs/auth-portal-ua.md) are documented. Portal implementation and automatic role grants remain future work. Evidence currently consists of actual command output; screenshots remain to be added for submission. Checks confirm isolation after readiness, not uninterrupted blocking during recreation. Cross-host portability has not been tested; the recorded run used OrbStack on macOS.

## Lab 14 Implementation Freeze

1. Commit the final version, run `git rev-parse HEAD`, and record its full SHA and date in the Freeze Record.
2. Run a short smoke check of that commit: `docker compose up -d --build --wait`, `docker compose ps`, `sh scripts/check-isolation.sh`, `sh scripts/check-internet.sh`. Save the output. Repeat the full restart/recreation checks if network code changes or the smoke check fails.
3. Fill the supplied Freeze Record with the baseline, requirement readiness, smoke results, known issues and testing handoff. Each participant completes their own Contribution Log #4 using their actual contribution.

R-01 is ready within the verified Guest web scope; R-02 and R-07 are ready for the tested Corporate HTTP restrictions. R-04 has a completed description but a missing implementation; R-08 still has a gap for dynamic role-based access. Match the exact requirement wording against the team's Requirements document.

After a successful smoke check, use `READY WITH KNOWN ISSUES` for network testing; use `HOLD` if the check fails. Start Lab 15 with TC-01 (Guest internet), TC-02 (Guest restriction), and TC-07 (IoT restriction). Login, role, logout and session-expiry tests remain blocked until the portal and gateway integration exist.
