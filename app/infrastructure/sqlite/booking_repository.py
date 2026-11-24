"""Adaptateur du port BookingRepository : c'est ici que vit la garantie de non-surréservation.

Décrément conditionnel sous BEGIN IMMEDIATE + index unique partiel sur la réservation active
(voir docs/adr/0004-concurrence-sqlite.md).
"""
from __future__ import annotations

import sqlite3
from typing import Optional

from app.domain.errors import DuplicateBooking, RideFull
from app.domain.models import Booking
from app.infrastructure.sqlite.database import Database


def _to_booking(row: sqlite3.Row) -> Booking:
    return Booking(row["id"], row["ride_id"], row["passenger"], row["status"], row["created_at"])


class SqliteBookingRepository:
    def __init__(self, db: Database):
        self.db = db

    def book_seat(self, ride_id: int, passenger: str, created_at: str) -> Booking:
        conn = self.db.connect()
        try:
            conn.execute("BEGIN IMMEDIATE")
            cur = conn.execute(
                "UPDATE rides SET seats_available = seats_available - 1"
                " WHERE id = ? AND seats_available > 0", (ride_id,))
            if cur.rowcount == 0:
                conn.execute("ROLLBACK")
                raise RideFull(f"le trajet {ride_id} est complet")
            try:
                cur = conn.execute(
                    "INSERT INTO bookings(ride_id, passenger, status, created_at)"
                    " VALUES (?, ?, 'ACTIVE', ?)", (ride_id, passenger, created_at))
            except sqlite3.IntegrityError:
                conn.execute("ROLLBACK")
                raise DuplicateBooking("ce passager a déjà une réservation sur ce trajet")
            booking_id = cur.lastrowid
            conn.execute("COMMIT")
        finally:
            conn.close()
        return self.get(booking_id)

    def get(self, booking_id: int) -> Optional[Booking]:
        conn = self.db.connect()
        try:
            row = conn.execute("SELECT * FROM bookings WHERE id = ?", (booking_id,)).fetchone()
        finally:
            conn.close()
        return _to_booking(row) if row else None
