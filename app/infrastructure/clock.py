"""Adaptateur du port Clock : horloge système en UTC."""
from __future__ import annotations

from datetime import datetime, timezone


class SystemClock:
    def now(self) -> str:
        return datetime.now(timezone.utc).isoformat()
