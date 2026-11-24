"""Erreurs métier. Aucune notion de transport : la correspondance HTTP vit dans app/api."""
from __future__ import annotations


class DomainError(Exception):
    """Base des erreurs métier."""

    def __init__(self, message: str | None = None):
        super().__init__(message or self.__class__.__name__)


class RideNotFound(DomainError):
    """Le trajet demandé n'existe pas."""


class RideFull(DomainError):
    """Aucune place disponible sur le trajet."""


class DuplicateBooking(DomainError):
    """Le passager détient déjà une réservation active sur ce trajet."""


class DriverCannotBook(DomainError):
    """Un conducteur ne peut pas réserver sur son propre trajet."""


class InvalidRide(DomainError):
    """Trajet invalide (champ manquant ou incohérent). Réservé à UC1 / UC9."""
