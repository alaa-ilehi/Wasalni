class AppError(Exception):
    """Base class for business errors. `status` is the HTTP code used by the controller."""
    status = 400

    def __init__(self, message=None):
        super().__init__(message or self.__class__.__name__)


class Unauthorized(AppError):
    status = 401


class RideNotFound(AppError):
    status = 404


class RideFull(AppError):
    status = 409


class DuplicateBooking(AppError):
    status = 409


class InvalidBooking(AppError):
    status = 400
