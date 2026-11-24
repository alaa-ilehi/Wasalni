"""Erreurs applicatives : elles concernent l'orchestration, pas les règles métier."""
from __future__ import annotations


class ApplicationError(Exception):
    """Base des erreurs applicatives."""

    def __init__(self, message: str | None = None):
        super().__init__(message or self.__class__.__name__)


class IdentityMissing(ApplicationError):
    """Aucune identité n'a été fournie par la frontière (en-tête X-User absent)."""
