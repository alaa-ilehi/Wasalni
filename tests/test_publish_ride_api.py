"""UC1 - Publier un trajet : validation, réponse 201 et intégration avec UC3."""
import sqlite3
from datetime import datetime, timedelta, timezone

import pytest

from app import create_app

DEPARTURE = "2035-01-15T08:00:00+00:00"
HEADERS = {"X-User": "driver1"}


def payload(**changes):
    data = {
        "origin": "Tunis",
        "destination": "Sfax",
        "departure_time": DEPARTURE,
        "seats_total": 3,
        "price": 20.0,
    }
    data.update(changes)
    return data


def test_publish_ride_nominal_scenario(app, client):
    r = client.post("/rides", json=payload(), headers=HEADERS)

    assert r.status_code == 201
    data = r.get_json()
    assert data["id"] > 0
    assert data["driver"] == data["driver_id"] == "driver1"
    assert data["origin"] == "Tunis"
    assert data["destination"] == "Sfax"
    assert data["departure"] == data["departure_time"] == DEPARTURE
    assert data["seats_total"] == 3
    assert data["seats_available"] == 3
    assert data["price"] == 20.0
    assert data["cancel_delay_hours"] == 2          # valeur par défaut
    assert data["status"] == "PUBLISHED"

    stored = app.container.rides.get(data["id"])
    assert stored.seats_available == stored.seats_total
    assert stored.status.value == "PUBLISHED"


@pytest.mark.parametrize("changes, message", [
    ({"origin": ""}, "l'origine est obligatoire"),
    ({"destination": "   "}, "la destination est obligatoire"),
    ({"destination": "tunis"}, "l'origine et la destination doivent être différentes"),
    ({"seats_total": 0}, "le nombre de places doit être au moins 1"),
    ({"departure_time": "2020-01-01T08:00:00+00:00"}, "le départ doit être dans le futur"),
    ({"price": -1}, "le prix ne peut pas être négatif"),
    ({"cancel_delay_hours": -1}, "le délai d'annulation ne peut pas être négatif"),
    ({"seats_total": "deux"}, "le nombre de places doit être un entier"),
    ({"price": "gratuit"}, "le prix doit être un nombre"),
    ({"departure_time": "pas-une-date"}, "le départ doit être une date au format ISO 8601"),
])
def test_invalid_ride_returns_400_with_french_message(client, changes, message):
    r = client.post("/rides", json=payload(**changes), headers=HEADERS)

    assert r.status_code == 400
    assert r.get_json() == {"error": message}


def test_cancel_delay_must_be_shorter_than_time_left(client):
    soon = (datetime.now(timezone.utc) + timedelta(minutes=30)).isoformat()

    r = client.post("/rides", json=payload(departure_time=soon), headers=HEADERS)

    assert r.status_code == 400
    assert r.get_json() == {
        "error": "le délai d'annulation doit être inférieur au temps restant avant le départ"
    }


def test_explicit_cancel_delay_lower_than_time_left_is_accepted(client):
    soon = (datetime.now(timezone.utc) + timedelta(hours=3)).isoformat()

    r = client.post("/rides", json=payload(departure_time=soon, cancel_delay_hours=1),
                    headers=HEADERS)

    assert r.status_code == 201
    assert r.get_json()["cancel_delay_hours"] == 1


def test_non_json_body_returns_400(client):
    r = client.post("/rides", data="pas du json", content_type="text/plain", headers=HEADERS)

    assert r.status_code == 400
    assert r.get_json() == {"error": "le corps de la requête doit être un objet JSON"}


def test_departure_field_is_also_accepted(client):
    data = payload()
    data.pop("departure_time")
    data["departure"] = "2035-02-01T08:00"

    r = client.post("/rides", json=data, headers=HEADERS)

    assert r.status_code == 201
    assert r.get_json()["departure"] == "2035-02-01T08:00"


def test_publish_ride_without_x_user_returns_401(client):
    r = client.post("/rides", json=payload())

    assert r.status_code == 401
    assert r.get_json() == {"error": "en-tête X-User absent"}


def test_published_ride_can_be_booked_through_uc3(app, client, notifier):
    created = client.post("/rides", json=payload(seats_total=2), headers=HEADERS)
    assert created.status_code == 201
    ride_id = created.get_json()["id"]

    booking = client.post(f"/rides/{ride_id}/bookings", headers={"X-User": "ali"})

    assert booking.status_code == 201
    assert booking.get_json()["status"] == "ACTIVE"
    assert app.container.rides.get(ride_id).seats_available == 1
    assert len(notifier.sent) == 1


def test_existing_database_gets_uc1_columns_and_keeps_uc3(tmp_path, notifier):
    db_path = tmp_path / "legacy.db"
    conn = sqlite3.connect(db_path)
    conn.execute(
        "CREATE TABLE rides (id INTEGER PRIMARY KEY AUTOINCREMENT, driver TEXT NOT NULL,"
        " origin TEXT NOT NULL, destination TEXT NOT NULL, departure TEXT NOT NULL,"
        " seats_total INTEGER NOT NULL, seats_available INTEGER NOT NULL, price REAL NOT NULL)")
    conn.execute(
        "INSERT INTO rides(driver, origin, destination, departure, seats_total,"
        " seats_available, price) VALUES ('driver1', 'Tunis', 'Sfax', '2030-01-15T08:00', 2, 2, 20)")
    conn.commit()
    conn.close()

    legacy_app = create_app(str(db_path), notifier)
    ride = legacy_app.container.rides.get(1)
    assert ride.cancel_delay_hours == 2
    assert ride.status.value == "PUBLISHED"

    booking = legacy_app.test_client().post("/rides/1/bookings", headers={"X-User": "ali"})
    assert booking.status_code == 201
    assert legacy_app.container.rides.get(1).seats_available == 1
