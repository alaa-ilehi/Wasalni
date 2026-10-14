# UC3 - Book a seat

- **Primary actor:** Passenger  **Secondary actor:** Notification service
- **Preconditions:** the ride exists (user identified by the `X-User` header).
- **Postconditions (success):** one seat is held; a PENDING booking exists; the passenger is told to pay (UC4).

## Nominal scenario
1. The passenger sends `POST /rides/{id}/bookings`.
2. The system checks that the ride exists.
3. The system checks that the passenger is not the driver.
4. The system atomically decrements `seats_available` (only if > 0) and creates a PENDING booking.
5. The system notifies the passenger.
6. The system answers `201 Created`.

## Alternative scenarios
| # | Condition | Result |
|---|---|---|
| A1 | No identity | `401` |
| A2 | Ride unknown | `404` |
| A3 | Passenger is the driver | `400` |
| A4 | No seat left (including a concurrent request taking the last one) | `409`, nothing written |
| A5 | Passenger already has an active (PENDING/CONFIRMED) booking on this ride | `409`, seat not consumed |

## Business rules
- `seats_available` never goes below 0 (SQL `CHECK` + conditional `UPDATE`).
- One active booking per passenger and ride (partial unique index).
- Seat decrement and booking insert happen in one transaction.

## Tests
- Unit (fake repositories): `tests/test_booking_service_unit.py`
- Integration (nominal scenario): `test_book_seat_nominal_scenario` in `tests/test_booking_api.py`
- Risk: `tests/test_concurrency.py` (15 parallel requests, 3 seats: exactly 3 succeed)
