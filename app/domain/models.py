"""Entités du domaine et invariants qu'elles portent."""
from __future__ import annotations

from dataclasses import dataclass
from enum import Enum


class BookingStatus(str, Enum):
    """Une réservation est active, ou annulée. Aucun état intermédiaire (ADR 0003)."""

    ACTIVE = "ACTIVE"
    CANCELLED = "CANCELLED"


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

    def __post_init__(self) -> None:
        if self.seats_total < 1:
            raise ValueError("seats_total must be >= 1")
        if not 0 <= self.seats_available <= self.seats_total:
            raise ValueError("seats_available must be within 0..seats_total")

    def to_dict(self) -> dict:
        return {
            "id": self.id,
            "driver": self.driver,
            "origin": self.origin,
            "destination": self.destination,
            "departure": self.departure,
            "seats_total": self.seats_total,
            "seats_available": self.seats_available,
            "price": self.price,
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
