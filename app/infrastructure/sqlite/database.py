"""Fabrique de connexions SQLite (ADR 0005).

Une connexion par opération, autocommit désactivé : les transactions sont ouvertes explicitement
par les adaptateurs (`BEGIN IMMEDIATE`, voir ADR 0004).
"""
from __future__ import annotations

import sqlite3

from app.infrastructure.sqlite.schema import SCHEMA


class Database:
    def __init__(self, path: str):
        self.path = path
        self.apply_schema()

    def apply_schema(self) -> None:
        conn = self.connect()
        try:
            conn.executescript(SCHEMA)
        finally:
            conn.close()

    def connect(self) -> sqlite3.Connection:
        conn = sqlite3.connect(self.path, timeout=30, isolation_level=None)
        conn.row_factory = sqlite3.Row
        conn.execute("PRAGMA foreign_keys = ON")
        return conn
