"""Integration tests: HTTP -> controller -> service -> repository -> SQLite."""
from tests.conftest import make_ride


def test_book_seat_nominal_scenario(app, client, notifier):
    ride_id = make_ride(app, seats=2)
    r = client.post(f"/rides/{ride_id}/bookings", headers={"X-User": "ali"})
    assert r.status_code == 201
    assert r.get_json()["status"] == "PENDING"
    assert app.rides.get(ride_id).seats_available == 1          # seat decremented in the database
    assert len(notifier.sent) == 1


def test_last_seat_second_passenger_gets_409(app, client):
    ride_id = make_ride(app, seats=1)
    first = client.post(f"/rides/{ride_id}/bookings", headers={"X-User": "ali"})
    second = client.post(f"/rides/{ride_id}/bookings", headers={"X-User": "sara"})
    assert first.status_code == 201
    assert second.status_code == 409
    assert app.rides.get(ride_id).seats_available == 0


def test_unknown_ride_404(client):
    assert client.post("/rides/999/bookings", headers={"X-User": "ali"}).status_code == 404


def test_missing_user_401(app, client):
    ride_id = make_ride(app)
    assert client.post(f"/rides/{ride_id}/bookings").status_code == 401


def test_driver_cannot_book_own_ride_400(app, client):
    ride_id = make_ride(app, driver="driver1")
    r = client.post(f"/rides/{ride_id}/bookings", headers={"X-User": "driver1"})
    assert r.status_code == 400


def test_double_booking_same_passenger_409(app, client):
    ride_id = make_ride(app, seats=3)
    client.post(f"/rides/{ride_id}/bookings", headers={"X-User": "ali"})
    r = client.post(f"/rides/{ride_id}/bookings", headers={"X-User": "ali"})
    assert r.status_code == 409
    assert app.rides.get(ride_id).seats_available == 2          # failed attempt did not consume a seat
