# Sprint 1: Network Foundation

## Goal

Prepare a basic Docker Compose model with separate Guest, Corporate, and IoT segments, record their network parameters, select an approach for modeling role-based access, and agree on the evidence format for subsequent testing.

## Traceability

- **R-09:** Prepare distinct subnets for Guest, Corporate, and IoT.
- **D-01:** Establish logical zones and a gateway for future inter-zone traffic control.
- **D-02:** Select a local authentication and role-modeling approach for Student, Teacher, and Admin.

These references follow the supplied Sprint 1 record. The underlying Requirements and Design documents have not yet been reviewed in this repository.

## Work items

| ID | Work item | Result | Status |
| --- | --- | --- | --- |
| W-01 | Prepare Docker Compose networks, representative containers, and a basic gateway. | Three networks and four services configured in `docker-compose.yml`; Guest and IoT access to the Corporate site verified. | DONE |
| W-02 | Define how the portal identifies users and represents roles. | Local `/login`, SQLite test accounts, and a role-display flow selected. Individual role permissions remain an open decision. | DONE (approach selected) |
| W-03 | Agree on a common evidence format. | Command, expected result, actual result, status, and evidence source recorded below. | DONE |

## Implemented topology

| Segment | Subnet | Docker bridge gateway | Lab gateway | Service / IP |
| --- | --- | --- | --- | --- |
| Guest | `172.28.10.0/24` | `172.28.10.1` | `172.28.10.254` | `guest-client`: `172.28.10.10` |
| Corporate | `172.28.20.0/24` | `172.28.20.1` | `172.28.20.254` | `internal-site`: `172.28.20.10` |
| IoT | `172.28.30.0/24` | `172.28.30.1` | `172.28.30.254` | `iot-client`: `172.28.30.10` |

The Compose project is named `university-network`. The gateway and test clients use `alpine:3.22`; the internal site uses `nginx:stable-alpine`.

The lab gateway connects to all three segments and enables `net.ipv4.ip_forward=1`. Each client and the internal site replaces its default route with the `.254` address in its own segment on startup. The `.1` addresses belong to Docker's bridge gateways and are distinct from the lab gateway.

All four services have `NET_ADMIN` capability for route or firewall management. No application ports are published to the host. No custom firewall isolation rules or internet NAT rules have been added.

## Selected authentication approach

- A local JavaScript frontend at `/login` will provide username and password fields, **Sign in**, and **Continue as guest**.
- A portal backend will check test accounts stored in SQLite with password hashes and roles: Student, Teacher, or Admin.
- Successful login will display the account's role. Guest access will remain unauthenticated.
- The full model will associate a session with a client IP and request network permissions from the gateway. The permission matrix and gateway control interface are still to be designed.

This sprint selects the approach; no portal, database, login session, or role-based firewall integration has been implemented.

## Validation in this sprint

- Validate the Docker Compose configuration and start the foundation containers.
- Check that each container is attached to the intended network and has the expected IP address.
- Inspect the gateway interfaces and routing configuration.
- Record the results of the checks actually performed.

These checks establish the topology. They do not establish that guest isolation or role-based network enforcement works.

## Scope boundaries

The required role work in this sprint is to select and describe the implementation approach. A working login page that displays a role is an optional extension, not a prerequisite for completing the network foundation.

The following are later implementation targets:

- Enforcing inter-segment isolation rules.
- Verifying Guest-to-Internet access and enforcing the intended Guest-to-Internal-Services and IoT-to-Corporate restrictions. Successful internal HTTP requests in this sprint establish only baseline connectivity.
- Updating gateway permissions after login.
- Revoking permissions on logout or session expiry.
- Demonstrating distinct network permissions for authenticated roles.

No separate report document is planned in this repository. Project explanation and sprint scope are maintained in Markdown.

## Validation results and evidence

Validation was performed by the project author in a local Docker environment on 20 October 2026. The HTTP and ping outputs were pasted into the project chat; route, forwarding, configuration, and startup checks were reported as successful without full output. No separate log files or screenshots have been saved in the repository.

| Check ID | Action / command | Expected result | Actual result | Status | Evidence source |
| --- | --- | --- | --- | --- | --- |
| NET-01 | `docker compose config --quiet` and `docker compose up -d` | Configuration accepted and services start. | Author reported successful setup. | PASS (reported) | Author confirmation in chat. |
| NET-02 | `docker compose exec guest-client ip route` | Default route via `172.28.10.254`. | Author reported the route check succeeded. | PASS (reported) | Author confirmation in chat. |
| NET-03 | `docker compose exec internal-site ip route` | Default route via `172.28.20.254`. | Author reported the route check succeeded. | PASS (reported) | Author confirmation in chat. |
| NET-04 | `docker compose exec gateway sysctl net.ipv4.ip_forward` | `net.ipv4.ip_forward = 1`. | Author reported the forwarding check succeeded. | PASS (reported) | Author confirmation in chat. |
| NET-05 | `docker compose exec guest-client wget -qO- http://172.28.20.10` | Nginx welcome page returned. | HTML containing `Welcome to nginx!` returned. | PASS | Terminal output pasted in chat. |
| NET-06 | `docker compose exec iot-client ping -c 3 172.28.30.254` | Gateway responds to three probes. | 3 packets transmitted, 3 received; 0% packet loss. | PASS | Terminal output pasted in chat. |
| NET-07 | `docker compose exec iot-client wget -qO- http://172.28.20.10` | Nginx welcome page returned. | HTML containing `Welcome to nginx!` returned. | PASS | Terminal output pasted in chat. |

Guest and IoT access to Corporate is expected for this unrestricted baseline. These results do not demonstrate authentication, isolation, or internet access.

Use the following format for additional checks:

| Check ID | Action / command | Expected result | Actual result | PASS / FAIL | Log / screenshot reference |
| --- | --- | --- | --- | --- | --- |

Populate evidence only after running a check. Unexecuted checks must not be reported as passing.

## Open decisions

- The selected subnets work in the tested local environment; check for host or VPN overlap when running elsewhere.
- The gateway image is Alpine; `nftables` remains the proposed firewall tooling and is not installed or configured by this Compose file.
- Agree on the resource and port permissions for Student, Teacher, and Admin before implementing role-based firewall rules.
- Select the restricted portal-to-gateway control mechanism in a later implementation step.

