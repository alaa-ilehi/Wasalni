"""Ports : contrats que l'application utilise et que l'infrastructure implémente.

Un port n'existe que s'il a un système externe derrière lui ou plusieurs implémentations réelles
(production et test). Voir docs/adr/0001-couches-et-ports.md.
"""
from __future__ import annotations

from typing import Optional, Protocol, runtime_checkable

from app.domain.models import Booking, Ride


@runtime_checkable
class RideRepository(Protocol):
    def create(self, driver: str, origin: str, destination: str, departure: str,
               seats: int, price: float) -> Ride: ...

    def get(self, ride_id: int) -> Optional[Ride]: ...


@runtime_checkable
class BookingRepository(Protocol):
    def book_seat(self, ride_id: int, passenger: str, created_at: str) -> Booking:
        """Réserve une place de façon atomique.

        Lève RideFull si plus aucune place n'est disponible et DuplicateBooking si le passager
        détient déjà une réservation active sur ce trajet. Une réservation refusée ne consomme
        aucune place.
        """
        ...

    def get(self, booking_id: int) -> Optional[Booking]: ...


@runtime_checkable
class NotificationPort(Protocol):
    def send(self, recipient: str, message: str, event_type: str) -> None:
        """Transmet un message au système de notification externe.

        Livraison au mieux : un échec est journalisé et ne remonte jamais à l'appelant.
        """
        ...


@runtime_checkable
class Clock(Protocol):
    def now(self) -> str:
        """Instant courant en ISO 8601 UTC."""
        ...
