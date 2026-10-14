import pytest

from app import create_app
from app.services.notification_service import NotificationService


@pytest.fixture
def notifier():
    return NotificationService()


@pytest.fixture
def app(tmp_path, notifier):
    return create_app(str(tmp_path / "test.db"), notifier)


@pytest.fixture
def client(app):
    return app.test_client()


def make_ride(app, driver="driver1", seats=2):
    """Seed a ride directly in the database (UC1 'Post a ride' is not built yet)."""
    return app.rides.create(driver, "Tunis", "Sfax", "2030-01-15T08:00", seats, 20.0).id
