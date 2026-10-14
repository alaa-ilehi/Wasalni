from app.errors import InvalidBooking, RideNotFound


class BookingService:
    """Business rules of UC3 (book a seat)."""

    def __init__(self, rides, bookings, notifier):
        self.rides = rides
        self.bookings = bookings
        self.notifier = notifier

    def book_seat(self, ride_id, passenger):
        ride = self.rides.get(ride_id)
        if ride is None:
            raise RideNotFound(f"ride {ride_id} not found")
        if ride.driver == passenger:
            raise InvalidBooking("a driver cannot book a seat on their own ride")
        booking = self.bookings.book_seat(ride_id, passenger)   # may raise RideFull / DuplicateBooking
        self.notifier.notify(passenger, f"Seat held on ride #{ride_id}: pay booking #{booking.id} to confirm")
        return booking
