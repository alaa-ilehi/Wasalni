"""DDL du périmètre implémenté (UC3).

Les tables des UC à venir (users, ratings, colonnes de UC1/UC11) sont décrites, mais non créées,
dans la section « modèle de données » de docs/architecture.md.
"""

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
    status      TEXT    NOT NULL DEFAULT 'ACTIVE',    -- ACTIVE | CANCELLED
    created_at  TEXT    NOT NULL
);
-- une seule réservation active par passager et par trajet (ADR 0004)
CREATE UNIQUE INDEX IF NOT EXISTS ux_active_booking
    ON bookings(ride_id, passenger) WHERE status = 'ACTIVE';
"""
