import sqlite3
from datetime import datetime, timezone

from app.errors import DuplicateBooking, RideFull
from app.models import Booking


def _now():
    return datetime.now(timezone.utc).isoformat()


class BookingRepository:
    def __init__(self, db):
        self.db = db

    def book_seat(self, ride_id, passenger):
        """Atomically hold one seat and create a PENDING booking.

        The decrement is a single conditional UPDATE inside a write transaction,
        so two concurrent requests can never take the last seat.
        """
        conn = self.db.connect()
        try:
            conn.execute("BEGIN IMMEDIATE")
            cur = conn.execute(
                "UPDATE rides SET seats_available = seats_available - 1"
                " WHERE id = ? AND seats_available > 0", (ride_id,))
            if cur.rowcount == 0:
                conn.execute("ROLLBACK")
                raise RideFull(f"ride {ride_id} is full")
            try:
                cur = conn.execute(
                    "INSERT INTO bookings(ride_id, passenger, status, created_at)"
                    " VALUES (?, ?, 'PENDING', ?)", (ride_id, passenger, _now()))
            except sqlite3.IntegrityError:
                conn.execute("ROLLBACK")
                raise DuplicateBooking("passenger already has a booking on this ride")
            booking_id = cur.lastrowid
            conn.execute("COMMIT")
        finally:
            conn.close()
        return self.get(booking_id)

    def get(self, booking_id):
        conn = self.db.connect()
        try:
            row = conn.execute("SELECT * FROM bookings WHERE id = ?", (booking_id,)).fetchone()
        finally:
            conn.close()
        return Booking.from_row(row) if row else None
