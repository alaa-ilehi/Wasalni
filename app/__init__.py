"""Racine de composition : instancie les adaptateurs, câble l'application, expose app.container.

Voir docs/architecture.md §3 etdocs/adr/0001-couches-et-ports.md.
"""
from __future__ import annotations

from dataclasses import dataclass

from flask import Flask

from app.api.http import bp
from app.application.book_seat import BookSeat
from app.infrastructure.clock import SystemClock
from app.infrastructure.notifications import RecordingNotifications
from app.infrastructure.sqlite.booking_repository import SqliteBookingRepository
from app.infrastructure.sqlite.database import Database
from app.infrastructure.sqlite.ride_repository import SqliteRideRepository


@dataclass
class Container:
    """Dépendances câblées, exposées pour les tests et pour les cas d'utilisation à venir."""

    database: Database
    rides: SqliteRideRepository
    bookings: SqliteBookingRepository
    notifications: RecordingNotifications
    clock: SystemClock
    book_seat: BookSeat


def create_app(db_path: str = "carpooling.db", notifications=None) -> Flask:
    database = Database(db_path)
    rides = SqliteRideRepository(database)
    bookings = SqliteBookingRepository(database)
    notifications = notifications or RecordingNotifications()
    clock = SystemClock()

    app = Flask(__name__)
    app.container = Container(
        database=database,
        rides=rides,
        bookings=bookings,
        notifications=notifications,
        clock=clock,
        book_seat=BookSeat(rides, bookings, notifications, clock),
    )
    app.register_blueprint(bp)
    return app
