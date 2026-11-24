"""Entités du domaine et invariants qu'elles portent."""
from __future__ import annotations

from dataclasses import dataclass
from enum import Enum


class BookingStatus(str, Enum):
    """Une réservation est active, ou annulée. Aucun état intermédiaire (ADR 0003)."""

    ACTIVE = "ACTIVE"
    CANCELLED = "CANCELLED"


class RideStatus(str, Enum):
    """Statut d'un trajet : publié, puis terminé (UC11)."""

    PUBLISHED = "PUBLISHED"
    COMPLETED = "COMPLETED"


@dataclass
class Ride:
    id: int
    driver: str
    origin: str
    destination: str
    departure: str          # ISO 8601
    seats_total: int
    seats_available: int
    price: float
    cancel_delay_hours: float = 2       # X de UC1, en heures
    status: RideStatus = RideStatus.PUBLISHED

    def __post_init__(self) -> None:
        if self.seats_total < 1:
            raise ValueError("seats_total must be >= 1")
        if not 0 <= self.seats_available <= self.seats_total:
            raise ValueError("seats_available must be within 0..seats_total")
        self.status = RideStatus(self.status)

    def to_dict(self) -> dict:
        return {
            "id": self.id,
            "driver": self.driver,
            "driver_id": self.driver,       # alias demandé par la fiche UC1
            "origin": self.origin,
            "destination": self.destination,
            "departure": self.departure,
            "departure_time": self.departure,  # alias demandé par la fiche UC1
            "seats_total": self.seats_total,
            "seats_available": self.seats_available,
            "price": self.price,
            "cancel_delay_hours": self.cancel_delay_hours,
            "status": self.status.value,
        }


@dataclass
class Booking:
    id: int
    ride_id: int
    passenger: str
    status: BookingStatus
    created_at: str         # ISO 8601 UTC

    def __post_init__(self) -> None:
        self.status = BookingStatus(self.status)

    def to_dict(self) -> dict:
        return {
            "id": self.id,
            "ride_id": self.ride_id,
            "passenger": self.passenger,
            "status": self.status.value,
            "created_at": self.created_at,
        }
