"""Adaptateur du port NotificationPort : système de notification externe simulé.

Il n'envoie rien : il enregistre le destinataire et le message, ce qui suffit aux tests et respecte
la règle « livraison au mieux, jamais de rollback » (UC12).
"""
from __future__ import annotations


class RecordingNotifications:
    def __init__(self):
        self.sent: list[tuple[str, str]] = []

    def send(self, recipient: str, message: str, event_type: str) -> None:
        # Le type d'événement est transmis au système externe ; l'enregistreur conserve le couple
        # (destinataire, message), qui est ce que vérifient les tests.
        self.sent.append((recipient, message))
