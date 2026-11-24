"""Adaptateur du port RideRepository."""
from __future__ import annotations

import sqlite3
from typing import Optional

from app.domain.models import Ride
from app.infrastructure.sqlite.database import Database


def _to_ride(row: sqlite3.Row) -> Ride:
    return Ride(row["id"], row["driver"], row["origin"], row["destination"], row["departure"],
                row["seats_total"], row["seats_available"], row["price"])


class SqliteRideRepository:
    def __init__(self, db: Database):
        self.db = db

    def create(self, driver: str, origin: str, destination: str, departure: str,
               seats: int, price: float) -> Ride:
        conn = self.db.connect()
        try:
            cur = conn.execute(
                "INSERT INTO rides(driver, origin, destination, departure, seats_total,"
                " seats_available, price) VALUES (?,?,?,?,?,?,?)",
                (driver, origin, destination, departure, seats, seats, price))
            ride_id = cur.lastrowid
        finally:
            conn.close()
        return self.get(ride_id)

    def get(self, ride_id: int) -> Optional[Ride]:
        conn = self.db.connect()
        try:
            row = conn.execute("SELECT * FROM rides WHERE id = ?", (ride_id,)).fetchone()
        finally:
            conn.close()
        return _to_ride(row) if row else None
