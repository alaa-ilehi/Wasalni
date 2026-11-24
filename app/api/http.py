"""Adaptateur HTTP : routage, identité, correspondance erreur -> code HTTP."""
from __future__ import annotations

from flask import Blueprint, current_app, jsonify, request
from werkzeug.exceptions import HTTPException

from app.application.errors import IdentityMissing
from app.domain.errors import (
    DomainError,
    DriverCannotBook,
    DuplicateBooking,
    InvalidRide,
    RideFull,
    RideNotFound,
)

bp = Blueprint("api", __name__)

# Politique HTTP en un seul endroit (ADR 0006) : le domaine ne connaît aucun code de statut.
STATUS_BY_ERROR: tuple[tuple[type, int], ...] = (
    (RideNotFound, 404),
    (RideFull, 409),
    (DuplicateBooking, 409),
    (DriverCannotBook, 400),
    (InvalidRide, 400),
)


def http_status(error: Exception) -> int:
    for kind, status in STATUS_BY_ERROR:
        if isinstance(error, kind):
            return status
    return 500


def current_actor() -> str:
    """L'identité vient du système d'authentification externe, via l'en-tête X-User (ADR 0002)."""
    actor = request.headers.get("X-User", "").strip()
    if not actor:
        raise IdentityMissing("en-tête X-User absent")
    return actor


@bp.app_errorhandler(IdentityMissing)
def handle_identity_missing(e: IdentityMissing):
    return jsonify(error=str(e)), 401


@bp.app_errorhandler(DomainError)
def handle_domain_error(e: DomainError):
    return jsonify(error=str(e)), http_status(e)


@bp.app_errorhandler(Exception)
def handle_unexpected(e: Exception):
    if isinstance(e, HTTPException):
        return e.get_response()
    current_app.logger.exception("erreur inattendue")
    return jsonify(error="erreur interne"), 500


@bp.post("/rides/<int:ride_id>/bookings")
def book_seat(ride_id: int):
    booking = current_app.container.book_seat.execute(ride_id, current_actor())
    return jsonify(booking.to_dict()), 201
