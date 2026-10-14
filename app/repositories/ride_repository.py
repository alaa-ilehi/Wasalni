from app.models import Ride


class RideRepository:
    def __init__(self, db):
        self.db = db

    def create(self, driver, origin, destination, departure, seats, price):
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

    def get(self, ride_id):
        conn = self.db.connect()
        try:
            row = conn.execute("SELECT * FROM rides WHERE id = ?", (ride_id,)).fetchone()
        finally:
            conn.close()
        return Ride.from_row(row) if row else None
