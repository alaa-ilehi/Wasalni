"""Adaptateur du port IdentityProvider : l'identité arrive dans l'en-tête X-User.

Le système d'authentification est externe (UC13) ; cet adaptateur ne fait que lire la preuve
d'identité transmise par le client. Voir docs/adr/0002-identite-externe.md.
"""
from __future__ import annotations

from flask import request

from app.application.errors import IdentityMissing


class HeaderIdentityProvider:
    def current_actor(self) -> str:
        actor = request.headers.get("X-User", "").strip()
        if not actor:
            raise IdentityMissing("en-tête X-User absent")
        return actor
