# ADR 0005 - Persistance SQLite sans ORM, pilote pur Go

- **Statut :** accepté (révisé lors du portage en Go)
- **Date :** 2026-11-24

## Contexte

Le TP ne dispose d'aucune infrastructure serveur. Le service doit démarrer en une commande (`go run ./cmd/wasalni`) et
tourner en CI, et les tests d'intégration doivent s'exécuter sur une base jetable.

Le portage en Go ajoute une contrainte de plus : la CI ne doit pas avoir besoin d'une chaîne de compilation C, sous peine
de rendre le binaire dépendant de la machine qui le construit.

## Décision

- `database/sql` avec le pilote **`modernc.org/sqlite`** (implémentation pure Go), importé anonymement dans
  `internal/infrastructure/sqlite/base.go`. **Sans cgo, sans ORM, sans outil de migration.**
- `Ouvrir(ctx, chemin)` ouvre (et crée) le fichier, applique `PRAGMA journal_mode = WAL`, `PRAGMA busy_timeout = 5000`
  et `PRAGMA foreign_keys = ON`, puis appelle `AppliquerSchema`.
- **Une seule connexion** : `SetMaxOpenConns(1)` et `SetMaxIdleConns(1)`. SQLite n'accepte qu'un écrivain à la fois ; une
  connexion unique sérialise les accès au lieu de laisser le pilote arbitrer des attentes.
- Le DDL est un script idempotent (`internal/infrastructure/sqlite/schema.go`, `CREATE TABLE IF NOT EXISTS users`),
  appliqué à chaque démarrage.
- Les transactions sont ouvertes explicitement par l'adaptateur (`BeginTx`), avec `Rollback` différé et `Commit` explicite.
- `created_at` et `updated_at` sont posés par la base (`strftime('%Y-%m-%dT%H:%M:%SZ', 'now')`) et relus par `RETURNING` :
  le service ne calcule pas ses horodatages.
- La conversion ligne → entité appartient à l'adaptateur, jamais au domaine (qui ne connaît ni `database/sql` ni SQL).

## Conséquences

- **Positif :** aucune dépendance native et aucune cgo, donc un binaire statique et une CI sans outillage C ; SQL visible
  et explicite ; transactions maîtrisées ; tests sur un fichier temporaire (`t.TempDir()`).
- **Négatif :** pas de migrations versionnées (le schéma évoluera avec UC1, UC3, UC5, UC6 et UC8) ; une seule instance
  possible ; les écritures sont mises en file par la connexion unique ; la conversion manuelle est répétitive.
- **Mitigation :** les évolutions de schéma sont documentées dans la section « modèle de données » de
  [architecture.md](../architecture.md), avec le DDL réel, et le schéma est idempotent pour ne jamais bloquer un
  démarrage.

## Alternatives écartées

- **`mattn/go-sqlite3`** : impose cgo, donc une chaîne de compilation C en CI et sur le poste de chaque développeur.
- **Un ORM (GORM et équivalents)** : abstraction et dépendance pour une table, et masque le SQL qui porte la décision de
  concurrence de l'ADR 0004.
- **PostgreSQL** : nécessite un service, hors du cadre du TP.
- **Un outil de migration dès maintenant** : utile à partir de plusieurs environnements déployés, pas ici.
