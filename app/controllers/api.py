from flask import Blueprint, current_app, jsonify, request

from app.errors import AppError, Unauthorized

bp = Blueprint("api", __name__)


def current_user():
    """No login UC in this project: the user is simply identified by the X-User header."""
    user = request.headers.get("X-User", "").strip()
    if not user:
        raise Unauthorized("missing X-User header")
    return user


@bp.errorhandler(AppError)
def handle_app_error(e):
    return jsonify(error=str(e)), e.status


@bp.post("/rides/<int:ride_id>/bookings")
def book_seat(ride_id):
    user = current_user()
    booking = current_app.booking_service.book_seat(ride_id, user)
    return jsonify(booking.to_dict()), 201
