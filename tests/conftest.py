import pytest

from app import create_app
from app.infrastructure.notifications import RecordingNotifications


@pytest.fixture
def notifier():
    return RecordingNotifications()


@pytest.fixture
def app(tmp_path, notifier):
    return create_app(str(tmp_path / "test.db"), notifier)


@pytest.fixture
def client(app):
    return app.test_client()


def make_ride(app, driver="driver1", seats=2):
    """Sème un trajet directement en base (UC1 « publier un trajet » n'est pas encore construit)."""
    return app.container.rides.create(driver, "Tunis", "Sfax", "2030-01-15T08:00", seats, 20.0).id
