# Vision - Carpooling

**Problem:** drivers travel with empty seats while passengers struggle to find affordable, reliable transport between cities.
**Users:** drivers and passengers.
**Value:** post or find a ride in a few clicks, hold a seat, pay online, and never overbook a car.
**Scope:** simple web API, one platform, intercity rides only, user identified by a header (no login feature).
**Success:** zero overbooked rides and every booking either paid and confirmed, or released.

## Actors
| Actor | Type | Role |
|---|---|---|
| Driver | primary | posts rides |
| Passenger | primary | searches, books, pays, cancels, rates |
| Payment service | secondary (mocked) | charges the passenger's card |
| Notification service | secondary (mocked) | informs driver and passenger |

## Use cases
| ID | Name | Notes |
|---|---|---|
| UC1 | Post a ride | |
| UC2 | Search rides | |
| UC3 | Book a seat | **technical risk 1**: last-seat race. Creates a PENDING booking holding the seat |
| UC4 | Pay for a booking | **technical risk 2**: external payment, failure handling. PENDING -> CONFIRMED |
| UC5 | Cancel a booking | frees the seat, refunds if paid |
| UC6 | Rate the driver | not implemented (out of TP scope) |

Booking and payment are two separate use cases: a seat is held first (UC3), then confirmed by payment (UC4).

## Prioritization (1 = low, 3 = high)
| UC | Business value | Risk | Architecture impact | Total | Order |
|---|---|---|---|---|---|
| UC3 Book a seat | 3 | 3 | 3 | 9 | 1 |
| UC4 Pay for a booking | 3 | 3 | 2 | 8 | 2 |
| UC1 Post a ride | 3 | 1 | 3 | 7 | 3 |
| UC2 Search rides | 3 | 1 | 2 | 6 | 4 |
| UC5 Cancel a booking | 2 | 2 | 1 | 5 | 5 |
| UC6 Rate the driver | 1 | 1 | 1 | 3 | 6 |

**Decision:** UC3 first (concurrency risk, shapes the core design), then UC4 (payment risk).

## Plan: 3 iterations, 4 phases
| Iteration | Content | Phase | Tag |
|---|---|---|---|
| 0 | Vision, UCs, repository, use case diagram | Inception | `v0.0-inception` |
| 1 | Baseline architecture, UC3 end to end, first test in CI | Elaboration | `v0.1-elaboration` |
| 2 | UC4 payment, UC1, UC2 | Construction | `v0.2-construction` |
| 3 | UC5, UC6, fixes, cross-review, delivery | Construction -> Transition | `v1.0-transition` |
