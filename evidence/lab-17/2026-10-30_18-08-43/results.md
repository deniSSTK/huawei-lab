# Lab 17 retest and regression

Run: 2026-10-30_18-08-43 Europe/Kiev.
Test TTL: 12 seconds. Restored deployment TTL: 300 seconds.

| ID | Result | Scenario | Evidence |
| --- | --- | --- | --- |
| RT-01 | PASS | NT-04: login page, wrong password and unauthenticated restrictions | RT-01.log |
| RT-02 | PASS | NT-04: full Student/Teacher/Admin network port matrix | RT-02.log |
| RT-03 | PASS | NT-04: independent device isolation and copied-cookie rejection | RT-03.log |
| RT-04 | PASS | NT-04: role downgrade invalidates old session and old grants | RT-04.log |
| RT-05 | PASS | NT-04: Logout closes an already established TCP connection | RT-05.log |
| RT-06 | PASS | NT-04: session TTL revokes network access | RT-06.log |
| RT-07 | PASS | NT-04: gateway TTL revokes access while Portal is stopped | RT-07.log |
| RT-08 | PASS | NT-04: CSRF and closed/authenticated management boundary | RT-08.log |
| RT-09 | PASS | NT-04: nft failure rejects login; recovery remains fail closed | RT-09.log |
| RG-01 | PASS | NT-01/02: alternate port denied, IoT stays isolated even during Admin session | RG-01.log |
| RG-02 | PASS | TC-01/02/07: Guest web and Guest/IoT isolation regression | RG-02.log |
| RG-03 | PASS | NT-03: gateway restart and full recreation; login and regression after readiness | RG-03.log |

Source snapshot: source-sha256.json. Expectations saved before execution.
Recovery checks cover state after readiness, not uninterrupted isolation during container recreation.
Session grants are IP-bound in a trusted-client lab. Host browser is not a simulated Guest device.
Exit status: 0.
