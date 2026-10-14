from dataclasses import dataclass


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

    @classmethod
    def from_row(cls, r):
        return cls(r["id"], r["driver"], r["origin"], r["destination"], r["departure"],
                   r["seats_total"], r["seats_available"], r["price"])

    def to_dict(self):
        return self.__dict__.copy()


@dataclass
class Booking:
    id: int
    ride_id: int
    passenger: str
    status: str
    created_at: str

    @classmethod
    def from_row(cls, r):
        return cls(r["id"], r["ride_id"], r["passenger"], r["status"], r["created_at"])

    def to_dict(self):
        return self.__dict__.copy()
