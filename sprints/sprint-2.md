# Sprint 2: Firewall Foundation

[Українська версія](sprint-2-ua.md)

## Goal and scope

Extend the Sprint 1 topology with repeatable Guest-to-Corporate and IoT-to-Corporate isolation. This implements the Firewall foundation for R-02, R-07 and D-01 from the supplied Sprint 2 template. Initial runtime evidence is recorded below; this is not a completed Sprint 2 Record or Implementation Freeze decision.

## Implementation

- `gateway/Dockerfile` installs nftables at image build time.
- `gateway/nftables.conf` owns only the `inet lab_filter` table. Loading it again replaces its rules atomically without flushing other tables, such as future NAT configuration.
- Docker Compose enables IPv4 forwarding. The entrypoint loads the rules before the gateway becomes healthy; configuration failure stops the gateway. Other services wait for its healthcheck on Compose startup. This startup ordering is not an atomic fail-closed guarantee during gateway recreation while existing clients are running; validate the policy after recreation before resuming tests.
- Guest (`172.28.10.0/24`) and IoT (`172.28.30.0/24`) packets to Corporate (`172.28.20.0/24`) are dropped before the established/related rule. Named counters identify the two restrictions.
- Other established/related traffic is accepted; all other forwarded traffic is dropped. No new inter-zone or internet connections are allowed in this foundation. Gateway-local input/output is unchanged, so clients can ping their gateway and the gateway can check Nginx directly.

The rules apply inside the gateway container, not to the host Firewall. This remains a trusted-client lab: client containers retain `NET_ADMIN` for route setup. It does not protect against a participant deliberately changing routes or spoofing an IP.

## Checks to run

```bash
docker compose config --quiet
docker compose up -d --build --wait
docker compose ps
sh scripts/check-isolation.sh
docker compose restart gateway
sh scripts/check-isolation.sh
docker compose up -d --force-recreate --wait
sh scripts/check-isolation.sh
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

## Teammate handoff

Detailed Ukrainian implementation briefs: [Sprint 2 handoff](sprint-2-handoff-ua.md).

1. **Guest internet:** configure routing, explicit Guest internet forwarding, return traffic, and NAT on the gateway. Preserve the Corporate restrictions and Default Deny. Add a separate NAT table so reloading `lab_filter` does not erase NAT. Verify an external request originating from `guest-client`; a gateway-only request does not prove Guest access. Keep DNS and Docker's embedded resolver behavior in mind when recording the path.
2. **Validation and evidence:** run the controls and isolation checks, then repeat after restart and recreation. After teammate 1 finishes, verify Guest internet too. Store actual terminal output/screenshots under `evidence/sprint-2/`, with command, expected result, actual result, PASS/FAIL, and evidence path. Record defects with reproduction steps.

Auth Portal implementation and automatic role grants remain outside this foundation. Finalize the portal description and role matrix separately; do not mark authentication requirements READY based on this Firewall alone.

## Validation state

The project author supplied terminal output on 26 October 2026. The gateway image built successfully, Compose startup completed with the gateway healthy, and the loaded nftables table shows the intended rules. Evidence: [initial terminal output](../evidence/sprint-2/initial-isolation-2026-10-26.txt).

| Check | Actual result | Status |
| --- | --- | --- |
| Internal Site in Corporate | Nginx welcome page returned. | PASS |
| Gateway to Internal Site | Nginx welcome page returned. | PASS |
| Guest and IoT to their gateway | Each ping returned 1/1 replies with 0% loss. | PASS |
| Guest to Internal Site | HTTP timeout; guest_to_corporate packets increased from 0 to 3. | PASS |
| IoT to Internal Site | HTTP timeout; iot_to_corporate packets increased from 0 to 3. | PASS |

These are author-run results preserved from the supplied output, not an independent rerun. Restart/recreation checks, Guest internet configuration and verification, and terminal screenshots remain pending. Freeze readiness is not established by this initial run alone.
