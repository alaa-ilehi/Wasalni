import sqlite3

SCHEMA = """
CREATE TABLE IF NOT EXISTS rides (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    driver           TEXT    NOT NULL,
    origin           TEXT    NOT NULL,
    destination      TEXT    NOT NULL,
    departure        TEXT    NOT NULL,
    seats_total      INTEGER NOT NULL CHECK (seats_total >= 1),
    seats_available  INTEGER NOT NULL CHECK (seats_available >= 0),
    price            REAL    NOT NULL CHECK (price >= 0)
);
CREATE TABLE IF NOT EXISTS bookings (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ride_id     INTEGER NOT NULL REFERENCES rides(id),
    passenger   TEXT    NOT NULL,
    status      TEXT    NOT NULL DEFAULT 'PENDING',   -- PENDING | CONFIRMED | CANCELLED
    created_at  TEXT    NOT NULL
);
-- one active (pending or confirmed) booking per passenger and ride
CREATE UNIQUE INDEX IF NOT EXISTS ux_active_booking
    ON bookings(ride_id, passenger) WHERE status IN ('PENDING', 'CONFIRMED');
"""


class Database:
    """Thin SQLite wrapper. Every call gets its own autocommit connection;
    transactions are opened explicitly by the repositories (BEGIN IMMEDIATE)."""

    def __init__(self, path):
        self.path = path
        conn = self.connect()
        conn.executescript(SCHEMA)
        conn.close()

    def connect(self):
        conn = sqlite3.connect(self.path, timeout=30, isolation_level=None)
        conn.row_factory = sqlite3.Row
        conn.execute("PRAGMA foreign_keys = ON")
        return conn
