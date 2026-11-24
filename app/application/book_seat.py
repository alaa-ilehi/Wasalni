"""UC3 - Réserver une place.

Le cas d'utilisation orchestre les ports ; la garantie de non-surréservation appartient au port
BookingRepository (voir docs/adr/0004-concurrence-sqlite.md).
"""
from __future__ import annotations

from app.domain.errors import DriverCannotBook, RideNotFound
from app.domain.models import Booking
from app.domain.ports import BookingRepository, Clock, NotificationPort, RideRepository


class BookSeat:
    def __init__(self, rides: RideRepository, bookings: BookingRepository,
                 notifications: NotificationPort, clock: Clock):
        self.rides = rides
        self.bookings = bookings
        self.notifications = notifications
        self.clock = clock

    def execute(self, ride_id: int, actor: str) -> Booking:
        ride = self.rides.get(ride_id)
        if ride is None:
            raise RideNotFound(f"trajet {ride_id} introuvable")
        if ride.driver == actor:
            raise DriverCannotBook("un conducteur ne peut pas réserver sur son propre trajet")

        booking = self.bookings.book_seat(ride_id, actor, self.clock.now())
        self.notifications.send(
            actor,
            f"Place retenue sur le trajet #{ride_id} (réservation #{booking.id})",
            "booking.created",
        )
        return booking
