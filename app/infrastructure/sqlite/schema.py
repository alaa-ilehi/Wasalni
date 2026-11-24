"""DDL du périmètre implémenté (UC1 et UC3).

Les tables des UC à venir (users, ratings) sont décrites, mais non créées,
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
    price            REAL    NOT NULL CHECK (price >= 0),
    cancel_delay_hours REAL  NOT NULL DEFAULT 2 CHECK (cancel_delay_hours >= 0),
    status           TEXT    NOT NULL DEFAULT 'PUBLISHED'
        CHECK (status IN ('PUBLISHED', 'COMPLETED'))
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

# Colonnes ajoutées par UC1 à un trajet créé avant l'implémentation de UC1.
# Le DDL reste idempotent (ADR 0005) : pas de table de migration, juste un ALTER conditionnel.
RIDE_COLUMN_MIGRATIONS: tuple[tuple[str, str], ...] = (
    ("cancel_delay_hours", "cancel_delay_hours REAL NOT NULL DEFAULT 2 CHECK (cancel_delay_hours >= 0)"),
    ("status", "status TEXT NOT NULL DEFAULT 'PUBLISHED' CHECK (status IN ('PUBLISHED', 'COMPLETED'))"),
)
