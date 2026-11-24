"""Tests unitaires du cas d'utilisation BookSeat, avec des ports factices (scénarios UC3)."""
import pytest

from app.application.book_seat import BookSeat
from app.domain.errors import DriverCannotBook, DuplicateBooking, RideFull, RideNotFound
from app.domain.models import Booking, Ride
from app.infrastructure.notifications import RecordingNotifications


class FakeRides:
    def __init__(self, *rides):
        self.data = {r.id: r for r in rides}

    def get(self, ride_id):
        return self.data.get(ride_id)


class FakeBookings:
    def __init__(self, rides):
        self.rides, self.data = rides, {}

    def book_seat(self, ride_id, passenger, created_at):
        ride = self.rides.data[ride_id]
        if any(b.ride_id == ride_id and b.passenger == passenger and b.status == "ACTIVE"
               for b in self.data.values()):
            raise DuplicateBooking()
        if ride.seats_available == 0:
            raise RideFull()
        ride.seats_available -= 1
        b = Booking(len(self.data) + 1, ride_id, passenger, "ACTIVE", created_at)
        self.data[b.id] = b
        return b


class FixedClock:
    def now(self):
        return "2026-01-01T00:00:00+00:00"


@pytest.fixture
def setup():
    rides = FakeRides(Ride(1, "driver1", "Tunis", "Sfax", "2030-01-15T08:00", 1, 1, 20))
    notifier = RecordingNotifications()
    return BookSeat(rides, FakeBookings(rides), notifier, FixedClock()), rides, notifier


def test_book_seat_nominal(setup):
    service, rides, notifier = setup
    b = service.execute(1, "ali")
    assert b.status == "ACTIVE" and b.passenger == "ali"
    assert rides.data[1].seats_available == 0
    assert [u for u, _ in notifier.sent] == ["ali"]


def test_unknown_ride(setup):
    with pytest.raises(RideNotFound):
        setup[0].execute(99, "ali")


def test_driver_cannot_book_own_ride(setup):
    with pytest.raises(DriverCannotBook):
        setup[0].execute(1, "driver1")


def test_ride_full(setup):
    service = setup[0]
    service.execute(1, "ali")
    with pytest.raises(RideFull):
        service.execute(1, "sara")


def test_duplicate_booking(setup):
    service, rides, _ = setup
    rides.data[1].seats_available = 3
    service.execute(1, "ali")
    with pytest.raises(DuplicateBooking):
        service.execute(1, "ali")
