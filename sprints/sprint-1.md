# Sprint 1: Network Foundation

## Goal

Prepare a basic Docker Compose model with separate Guest, Corporate, and IoT segments, record their network parameters, select an approach for modeling role-based access, and agree on the evidence format for subsequent testing.

## Traceability

- **R-09:** Prepare distinct subnets for Guest, Corporate, and IoT.
- **D-01:** Establish logical zones and a gateway for future inter-zone traffic control.
- **D-02:** Select a local authentication and role-modeling approach for Student, Teacher, and Admin.

These references follow the supplied Sprint 1 record. The underlying Requirements and Design documents have not yet been reviewed in this repository.

## Work items

| ID | Work item | Expected output |
| --- | --- | --- |
| W-01 | Prepare Docker Compose networks, representative containers, and a basic gateway. | A starting topology with explicit subnets, IP addresses, and component network assignments. |
| W-02 | Define how the portal identifies users and represents roles. | A proposed local `/login` portal, test accounts backed by SQLite, and a role-display flow. Network permissions for individual roles remain to be agreed. |
| W-03 | Agree on a common evidence format. | A consistent way to capture actions, expected and actual results, PASS/FAIL outcomes, and supporting logs or screenshots. |

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
- Verifying Guest-to-Internet, Guest-to-Internal-Services, and IoT-to-Corporate flows.
- Updating gateway permissions after login.
- Revoking permissions on logout or session expiry.
- Demonstrating distinct network permissions for authenticated roles.

No separate report document is planned in this repository. Project explanation and sprint scope are maintained in Markdown.

## Evidence format

| Check ID | Action / command | Expected result | Actual result | PASS / FAIL | Log / screenshot reference |
| --- | --- | --- | --- | --- | --- |

Populate evidence only after running a check. Unexecuted checks must not be reported as passing.

## Open decisions

- Choose non-overlapping subnets that do not conflict with the host or VPN networks.
- Select the gateway image and firewall tooling; `nftables` is the current proposal.
- Agree on the resource and port permissions for Student, Teacher, and Admin before implementing role-based firewall rules.
- Select the restricted portal-to-gateway control mechanism in a later implementation step.

