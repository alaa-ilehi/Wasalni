from flask import Flask

from app.controllers.api import bp
from app.db import Database
from app.repositories.booking_repository import BookingRepository
from app.repositories.ride_repository import RideRepository
from app.services.booking_service import BookingService
from app.services.notification_service import NotificationService


def create_app(db_path="carpooling.db", notifier=None):
    app = Flask(__name__)
    db = Database(db_path)
    app.rides = RideRepository(db)          # exposed so tests can seed rides (UC1 comes later)
    bookings = BookingRepository(db)
    app.notifier = notifier or NotificationService()
    app.booking_service = BookingService(app.rides, bookings, app.notifier)
    app.register_blueprint(bp)
    return app
