# ADR 0005 - Persistance SQLite sans ORM

- **Statut :** accepté
- **Date :** 2026-11-24

## Contexte

Le TP ne dispose d'aucune infrastructure serveur. Le système doit tourner en une commande (`python run.py`) et en CI,
et les tests d'intégration doivent s'exécuter sur une base jetable.

## Décision

- `sqlite3` de la bibliothèque standard, **sans ORM ni couche de migration**.
- Le DDL est un script idempotent exécuté au démarrage de l'application (`app/infrastructure/sqlite/schema.py`).
- Une connexion par opération, autocommit désactivé (`isolation_level=None`), transactions ouvertes explicitement par
  les adaptateurs (`BEGIN IMMEDIATE`).
- `PRAGMA foreign_keys = ON` sur chaque connexion ; `sqlite3.Row` comme fabrique de lignes.
- La conversion ligne → entité appartient à l'adaptateur, jamais au domaine (qui ne connaît pas `sqlite3`).

## Conséquences

- **Positif :** zéro dépendance nouvelle, SQL visible et explicite, transactions maîtrisées, tests sur fichier temporaire
  (`tmp_path`) ou en mémoire.
- **Négatif :** pas de migrations versionnées (le schéma évoluera avec UC1/UC5/UC8/UC6) ; une seule instance possible ;
  la conversion manuelle est répétitive.
- **Mitigation :** les évolutions de schéma sont documentées dans la section « modèle de données » de
  [architecture.md](../architecture.md), avec un marquage explicite implémenté / cible.

## Alternatives écartées

- **SQLAlchemy** : abstraction et dépendance pour trois tables, et masque le `BEGIN IMMEDIATE` qui porte l'ADR 0004.
- **PostgreSQL** : nécessite un service, hors du cadre du TP.
- **Alembic dès maintenant** : utile à partir de plusieurs environnements déployés, pas ici.
