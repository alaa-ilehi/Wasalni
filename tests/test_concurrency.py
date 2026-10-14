"""The risky UC: many passengers race for the last seats."""
from concurrent.futures import ThreadPoolExecutor

from tests.conftest import make_ride


def test_no_overbooking_under_concurrency(app):
    seats, requests = 3, 15
    ride_id = make_ride(app, seats=seats)

    def attempt(i):
        return app.test_client().post(f"/rides/{ride_id}/bookings",
                                      headers={"X-User": f"user{i}"}).status_code

    with ThreadPoolExecutor(max_workers=requests) as pool:
        codes = list(pool.map(attempt, range(requests)))

    assert codes.count(201) == seats
    assert codes.count(409) == requests - seats
    assert app.rides.get(ride_id).seats_available == 0
