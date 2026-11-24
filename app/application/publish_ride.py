"""UC1 - Publier un trajet.

Les règles métier sont ici ; la frontière HTTP ne fait que transporter les champs et
le dépôt SQLite ne contient que du SQL (ADR 0001).
"""
from __future__ import annotations

from datetime import datetime, timezone
from typing import Any

from app.domain.errors import InvalidRide
from app.domain.models import Ride, RideStatus
from app.domain.ports import Clock, RideRepository

DEFAULT_CANCEL_DELAY_HOURS = 2


def _text(value: Any, field: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise InvalidRide(f"{field} est obligatoire")
    return value.strip()


def _number(value: Any, field: str) -> float:
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise InvalidRide(f"{field} doit être un nombre")
    return value


def _instant(value: Any, field: str) -> datetime:
    text = _text(value, field)
    try:
        parsed = datetime.fromisoformat(text.replace("Z", "+00:00"))
    except ValueError:
        raise InvalidRide(f"{field} doit être une date au format ISO 8601") from None
    return parsed if parsed.tzinfo else parsed.replace(tzinfo=timezone.utc)


class PublishRide:
    def __init__(self, rides: RideRepository, clock: Clock):
        self.rides = rides
        self.clock = clock

    def execute(self, driver: str, payload: Any) -> Ride:
        if not isinstance(payload, dict):
            raise InvalidRide("le corps de la requête doit être un objet JSON")

        origin = _text(payload.get("origin"), "l'origine")
        destination = _text(payload.get("destination"), "la destination")
        if origin.casefold() == destination.casefold():
            raise InvalidRide("l'origine et la destination doivent être différentes")

        seats_total = payload.get("seats_total")
        if isinstance(seats_total, bool) or not isinstance(seats_total, int):
            raise InvalidRide("le nombre de places doit être un entier")
        if seats_total < 1:
            raise InvalidRide("le nombre de places doit être au moins 1")

        price = _number(payload.get("price", 0), "le prix")
        if price < 0:
            raise InvalidRide("le prix ne peut pas être négatif")

        raw_departure = payload.get("departure_time", payload.get("departure"))
        departure = _instant(raw_departure, "le départ")
        now = _instant(self.clock.now(), "l'heure courante")
        if departure <= now:
            raise InvalidRide("le départ doit être dans le futur")

        delay = _number(payload.get("cancel_delay_hours", DEFAULT_CANCEL_DELAY_HOURS),
                        "le délai d'annulation")
        if delay < 0:
            raise InvalidRide("le délai d'annulation ne peut pas être négatif")
        hours_left = (departure - now).total_seconds() / 3600
        if delay >= hours_left:
            raise InvalidRide(
                "le délai d'annulation doit être inférieur au temps restant avant le départ")

        return self.rides.create(driver, origin, destination, raw_departure.strip(),
                                 seats_total, price, delay, RideStatus.PUBLISHED.value)
