"""UC3 - Réserver une place.

Le cas d'utilisation orchestre les ports ; la garantie de non-surréservation appartient au port
BookingRepository (voir docs/adr/0004-concurrence-sqlite.md), la livraison au mieux au port
NotificationPort (voir docs/adr/0007-notification-au-mieux.md).
"""
from __future__ import annotations

import logging

from app.domain.errors import DriverCannotBook, RideNotFound
from app.domain.models import Booking
from app.domain.ports import BookingRepository, Clock, NotificationPort, RideRepository

logger = logging.getLogger(__name__)


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
        self._notify(actor, f"Place retenue sur le trajet #{ride_id} (réservation #{booking.id})")
        return booking

    def _notify(self, actor: str, message: str) -> None:
        """Livraison au mieux : la réservation est déjà validée, un échec ne la remet pas en cause."""
        try:
            self.notifications.send(actor, message, "booking.created")
        except Exception:                                   # noqa: BLE001 - port externe
            logger.warning("notification non livrée à %s", actor, exc_info=True)
