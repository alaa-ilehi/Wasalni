"""Unit tests of BookingService with fake repositories (UC3 scenarios)."""
import pytest

from app.errors import DuplicateBooking, InvalidBooking, RideFull, RideNotFound
from app.models import Booking, Ride
from app.services.booking_service import BookingService
from app.services.notification_service import NotificationService


class FakeRides:
    def __init__(self, *rides):
        self.data = {r.id: r for r in rides}

    def get(self, ride_id):
        return self.data.get(ride_id)


class FakeBookings:
    def __init__(self, rides):
        self.rides, self.data = rides, {}

    def book_seat(self, ride_id, passenger):
        ride = self.rides.data[ride_id]
        if any(b.ride_id == ride_id and b.passenger == passenger and b.status != "CANCELLED"
               for b in self.data.values()):
            raise DuplicateBooking()
        if ride.seats_available == 0:
            raise RideFull()
        ride.seats_available -= 1
        b = Booking(len(self.data) + 1, ride_id, passenger, "PENDING", "now")
        self.data[b.id] = b
        return b


@pytest.fixture
def setup():
    rides = FakeRides(Ride(1, "driver1", "Tunis", "Sfax", "2030-01-15T08:00", 1, 1, 20))
    notifier = NotificationService()
    return BookingService(rides, FakeBookings(rides), notifier), rides, notifier


def test_book_seat_nominal(setup):
    service, rides, notifier = setup
    b = service.book_seat(1, "ali")
    assert b.status == "PENDING" and b.passenger == "ali"      # confirmed only after payment (UC4)
    assert rides.data[1].seats_available == 0
    assert [u for u, _ in notifier.sent] == ["ali"]


def test_unknown_ride(setup):
    with pytest.raises(RideNotFound):
        setup[0].book_seat(99, "ali")


def test_driver_cannot_book_own_ride(setup):
    with pytest.raises(InvalidBooking):
        setup[0].book_seat(1, "driver1")


def test_ride_full(setup):
    service = setup[0]
    service.book_seat(1, "ali")
    with pytest.raises(RideFull):
        service.book_seat(1, "sara")


def test_duplicate_booking(setup):
    service, rides, _ = setup
    rides.data[1].seats_available = 3
    service.book_seat(1, "ali")
    with pytest.raises(DuplicateBooking):
        service.book_seat(1, "ali")
