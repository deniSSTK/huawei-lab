# University Network Access Control Lab

[Українська версія](README-ua.md)

A Docker Compose proof of concept for separating guest, corporate, and IoT traffic in a university network. A local login portal will identify users, and a Linux gateway will enforce network access rules.

The Sprint 1 network foundation is implemented in [docker-compose.yml](docker-compose.yml): three networks, a Linux gateway, two test clients, and an internal Nginx site. Guest and IoT clients can currently reach the internal site. The portal, authentication, access restrictions, and internet NAT are not implemented. The sections below describe the full target design.

## Running the current network model

With Docker and Docker Compose available, run from the repository root:

```bash
docker compose config --quiet
docker compose up -d
docker compose ps
```

Check access to the internal site from each client:

```bash
docker compose exec guest-client wget -qO- http://172.28.20.10
docker compose exec iot-client wget -qO- http://172.28.20.10
```

Both requests should return the Nginx welcome page at this stage. These checks demonstrate baseline connectivity; no inter-segment isolation rules are configured yet. The site has no published host port, so these commands access it from inside the lab networks.

Stop and remove the lab containers and networks with:

```bash
docker compose down
```

See [Sprint 1](sprints/sprint-1.md) for addresses, routing details, and recorded validation results.

## Project idea

A client starts with guest access: it can reach the login portal and the internet, but cannot reach internal university services. The portal offers username and password fields, a **Sign in** button, and a **Continue as guest** button.

After a successful login, the portal displays the user's role: **Student**, **Teacher**, or **Admin**. In the full implementation, the portal also asks the gateway to grant that client the network permissions associated with the role. Logging out or reaching the session expiry time removes those permissions.

Guest is the default unauthenticated state. Student, Teacher, and Admin are authenticated user roles. Guest, Corporate, and IoT are network segments; roles and segments are separate concepts. Logging in does not move a client into another Docker network.

## Selected technology stack

The planned portal backend will use Go with the standard `net/http` package for HTTP handlers, credential checks, sessions, role lookup, and later gateway integration. The frontend will use plain HTML, CSS, and JavaScript for `/login` and the role page, without a frontend framework. SQLite will store test accounts, password hashes, and sessions.

The Go application will serve both the frontend files and the API from a single `portal` container. Routing and firewall enforcement will remain in the separate Linux gateway container. This stack is selected for future implementation; the portal is not part of the current running model.

## Components

| Component | Purpose |
| --- | --- |
| Client | A Docker container representing a user's device. It sends login requests and attempts to reach network resources. |
| Portal frontend | A JavaScript interface at `/login`, followed by a page showing the current role. |
| Portal backend | Checks credentials, creates sessions, associates a client IP with a user and role, and requests gateway permission changes. |
| SQLite database | Stores test accounts, password hashes, roles, and active session records. Plain-text passwords are not stored. |
| Gateway / firewall | A Linux container that routes traffic between segments and enforces rules, provisionally using `nftables`. |
| Internal resource | A separate university website in Corporate. A static page served by Nginx is sufficient to demonstrate network access control; a separate application backend is optional. |
| IoT client | A container used to test the isolation of IoT devices from Corporate services. |

## Network layout

```text
Guest network                       Corporate network
  Client ------ Gateway / Firewall ------ Internal resource
  Login portal          |
                        +------ IoT client (IoT network)
                        |
                        +------ Internet uplink
```

The portal must be reachable before authentication. Traffic from clients to internal resources must pass through the gateway. Client and resource containers must not share an alternative network that bypasses it. Internet traffic must also use the gateway when testing guest internet access.

Docker Compose assigns fixed IP addresses to lab clients. The current subnets, addresses, and routes are recorded in [Sprint 1](sprints/sprint-1.md). The gateway has `NET_ADMIN` capability and IPv4 forwarding enabled. Internet uplink behavior and NAT remain to be configured and tested.

This is a virtual network model. No physical Wi-Fi access point is required. Users open the portal explicitly; automatic captive-portal detection and redirection are outside the initial design.

## Login and network access flow

1. A client starts in Guest with access to the portal and the internet. Internal services are blocked by the gateway.
2. The client opens `/login`. Choosing **Continue as guest** keeps the existing guest permissions.
3. The user submits credentials. The backend verifies the password hash and loads the account's role from SQLite. The user cannot choose or override their role in the request.
4. The backend records the session, including the user, role, client IP, and expiry time.
5. Through a restricted control interface, the backend asks the gateway to apply the role's network permissions to that IP. Clients cannot access this control interface.
6. The portal displays the role. Subsequent requests to internal resources are allowed or blocked by the gateway's firewall rules.
7. Logout or session expiry revokes the gateway permissions and returns the client to guest access. Active grants must also expire on the gateway if the portal stops running.

The gateway does not inspect passwords or the role page. It checks packet source addresses, destination addresses, and ports against the installed rules. The portal translates an authenticated role into those rules.

## Where information lives

| Information | Location |
| --- | --- |
| Fixed client IP and network membership | Docker Compose configuration |
| Username, password hash, and role | Portal's SQLite database |
| User-to-IP association and session expiry | Portal's session records |
| Active network permissions for an IP | Gateway firewall rules or sets |

The portal must obtain the actual client IP from the connection. Any proxy or Docker NAT on that path must be accounted for before enabling IP-based grants.

For this lab, each simulated device has a distinct fixed IP and one active identity at a time. An IP-based grant applies to the whole device, including its other browser sessions. This simplified model does not address IP spoofing, shared NAT clients, or production network identity management.

## Roles and permissions

The initial role demonstration can show **Your role: Student**, **Your role: Teacher**, or **Your role: Admin** after login. This verifies authentication and role assignment, but does not by itself demonstrate network enforcement.

The minimum network demonstration distinguishes a guest from an authenticated client: the same internal resource is blocked before login and reachable after login. Showing different network permissions for Student, Teacher, and Admin requires an agreed resource and port matrix. That matrix is still to be decided; the role names alone do not define firewall rules.

IoT-to-Corporate isolation is a separate network policy and does not depend on a user signing in.

## How the full model will be tested

Login and resource requests must originate from the same simulated client so the gateway checks the IP that received the grant. A browser on the host can preview the portal, but does not automatically represent a Docker client.

| Scenario | Expected result |
| --- | --- |
| Guest accesses the portal | Portal is reachable. |
| Guest accesses the internet | Request succeeds through the gateway. |
| Guest accesses an internal resource | Gateway blocks the connection. |
| Each test account signs in | Portal displays the account's stored role. |
| Invalid credentials are submitted | Login fails and no network grant is created. |
| Authenticated client accesses an allowed internal resource | Connection succeeds. |
| A different client remains unauthenticated | Its access stays blocked. |
| Client logs out or its session expires | Internal access is blocked again. |
| IoT client accesses Corporate | Gateway blocks the connection under the isolation policy. |

Network enforcement will be verified using actual requests such as `curl` from client containers, together with firewall counters or logs. Returning an HTTP error from the portal alone does not prove that the network blocks direct access to the resource. Revocation tests must also check existing connections if the firewall allows established traffic.

Evidence will include the command or action, expected result, actual result, PASS/FAIL status, and supporting logs or screenshots.

## Sprint planning

Sprint scope is kept separately from the overall design:

- [Sprint overview](sprints/README.md)
- [Sprint 1: network foundation](sprints/sprint-1.md)

The full design above spans multiple sprints. Sprint 1 establishes the network foundation and selects the role-modeling approach. Inter-segment isolation tests and automatic firewall changes after login are subsequent work.
