# Architecture - Wasalni (covoiturage interurbain)

Document d'architecture du système décrit dans [vision.md](vision.md) et [use-case-elaboration.md](use-case-elaboration.md).
Il remplace l'organisation actuelle de `app/` : le code livré pour UC3 est repris comme *suggestion* de départ,
pas comme contrainte. L'écart entre les deux est explicité au §10, et les décisions structurantes sont tracées en ADR (§11).

## 1. Périmètre et objectifs

- **Périmètre fonctionnel :** les 12 cas d'utilisation de la vision. Seul UC3 est implémenté.
- **Objectifs d'architecture :**
  1. isoler les règles métier du transport (HTTP) et de la persistance (SQLite) ;
  2. rendre le risque technique du projet — la réservation concurrente — visible et testable à un seul endroit ;
  3. laisser une place explicite aux systèmes externes (authentification, notification) sans les simuler dans le domaine ;
  4. rendre l'ajout des UC restants mécanique : un cas d'utilisation = un fichier dans `application/` + des ports existants.
- **Hors périmètre :** toute validation externe différée, la messagerie temps réel, le déploiement multi-instance.

## 2. Vue de contexte

| Élément | Rôle vis-à-vis du système |
|---|---|
| Client (interface web en français) | consomme l'API HTTP, transporte l'identité dans l'en-tête `X-User` |
| Système d'authentification (externe) | émet l'identité ; le système ne stocke aucun mot de passe et n'émet aucun jeton |
| Système de notification (externe) | reçoit les événements de changement de statut ; livraison au mieux |
| Base SQLite | persistance locale des trajets, réservations et (à terme) profils et évaluations |

Le système ne dialogue jamais directement avec un utilisateur : tout passe par l'API. Les deux systèmes externes
sont des **ports** côté application (§5), jamais des dépendances du domaine.

## 3. Découpage en couches

Dépendances strictement orientées vers l'intérieur : `api` → `application` → `domain`, et `infrastructure` → `domain`
(elle implémente les ports déclarés par le domaine).

| Couche | Module | Responsabilité | Ne connaît pas |
|---|---|---|---|
| Domaine | `app/domain/` | entités (`Ride`, `Booking`), invariants, erreurs métier, **déclaration des ports** | Flask, SQL, HTTP |
| Application | `app/application/` | un cas d'utilisation par fichier : orchestration, règles applicatives, transactions via les ports | Flask, SQL |
| Infrastructure | `app/infrastructure/` | adaptateurs des ports : SQLite (schéma, dépôts, transactions), notification, horloge | Flask |
| Interface | `app/api/` | adaptateur HTTP : routage, extraction de l'identité, correspondance erreur → code HTTP | SQL, règles métier |
| Composition | `app/__init__.py` | racine de composition : instancie adaptateurs, câble l'application, expose `app.container` | — |

Règles de dépendance vérifiables par lecture des imports :

- `domain` n'importe rien de `app` hors `domain`.
- `application` importe `domain` uniquement.
- `infrastructure` importe `domain` (ports, entités) mais jamais `application` ni `api`.
- `api` importe `application` (cas d'utilisation, erreurs applicatives) et `domain` (ports, erreurs) mais jamais `infrastructure`.

## 4. Vue des composants

![Composants](uml/architecture-components.png)

Source : [uml/architecture-components.puml](uml/architecture-components.puml).

Chaîne nominale d'UC3 : `HttpApi` (adaptateur) → `BookSeat` (application) → ports `RideRepository`,
`BookingRepository`, `NotificationPort` → adaptateurs SQLite / notification.

Identité : `HttpApi` → port `IdentityProvider` → `HeaderIdentityProvider` (lit `X-User`). L'identité traverse le port,
pas le routeur (ADR 0002).

## 5. Ports et adaptateurs

| Port (déclaré dans `domain/ports.py`) | Contrat | Adaptateur retenu |
|---|---|---|
| `RideRepository` | `create(...) -> Ride`, `get(id) -> Ride \| None` | `infrastructure/sqlite/ride_repository.py` |
| `BookingRepository` | `book_seat(ride_id, passenger, created_at) -> Booking` (atomique), `get(id) -> Booking \| None` | `infrastructure/sqlite/booking_repository.py` |
| `NotificationPort` | `send(destinataire, message, type_evenement)` sans exception remontée | `infrastructure/notifications.py` (enregistreur, système externe simulé) |
| `Clock` | `now() -> str` (ISO 8601 UTC) | `infrastructure/clock.py` |
| `IdentityProvider` | `current_actor() -> str`, lève `IdentityMissing` si l'identité manque | `api/identity.py` (`HeaderIdentityProvider`, lit `X-User`) |

- **`Clock` existe** parce que UC5 aura besoin de comparer l'instant courant à `départ - X` et parce que les tests
  doivent pouvoir fixer le temps sans toucher au domaine.
- **`IdentityProvider` existe** parce que l'authentification est un système externe (UC13) : il évite que la frontière
  applicative dépendre d'un en-tête. Le contrat reçoit et rend des chaînes, jamais des objets de transport.
- Un port n'est ajouté que s'il a un **système externe derrière lui** (notification, authentification) ou **deux
  implémentations réelles** (dépôts : SQLite et factices de test). Sinon c'est une abstraction spéculative.

## 6. Modèle de données

Statut : **I** = implémenté, **C** = cible documentée pour les itérations suivantes (aucun code aujourd'hui).

```sql
rides (                                    -- I
  id, driver, origin, destination, departure, seats_total, seats_available, price,
  cancellation_window_minutes  -- C : X de UC1, ajouté avec UC1
  status                       -- C : PUBLISHED | COMPLETED | CANCELLED, ajouté avec UC11
)
bookings (                                 -- I
  id, ride_id, passenger, status, created_at
  -- status ∈ {ACTIVE, CANCELLED}
)
users (                                    -- C : UC8
  id, name, age, sex, photo_url
)
ratings (                                  -- C : UC6
  id, ride_id, author, target, score, comment, created_at
  -- unique(ride_id, author) : une évaluation par personne et par trajet
)
```

Invariants portés par le schéma ou par le domaine :

- `seats_available` entre 0 et `seats_total` (`CHECK`) ; jamais négatif.
- Une seule réservation `ACTIVE` par couple (trajet, passager) : index unique partiel `WHERE status = 'ACTIVE'`.
- Une réservation n'existe que pour un trajet existant : clé étrangère activée (`PRAGMA foreign_keys = ON`).
- Le domaine refuse une réservation sur son propre trajet (`DriverCannotBook`).

### Modèle de classes (périmètre implémenté)

![Modèle de classes](uml/class-model.png)

Source : [uml/class-model.puml](uml/class-model.puml). Le diagramme décrit le **code livré** : entités et statut de
réservation, deux hiérarchies d'erreurs séparées (`DomainError` pour le métier, `ApplicationError` pour l'application),
ports et leurs adaptateurs, et usage de `BookSeat`. Il se maintient avec le code : un diagramme qui diverge de
l'implémentation est un défaut, pas une illustration.

## 7. Concurrence et transactions

Le risque technique du projet est la course sur la dernière place. Il est traité **en un seul endroit** :
`BookingRepository.book_seat`, contrat de l'adaptateur SQLite.

1. `BEGIN IMMEDIATE` : le verrou d'écriture est pris au début de la transaction, ce qui évite la reprise en
   `SQLITE_BUSY` d'une transaction qui commencerait en lecture.
2. `UPDATE rides SET seats_available = seats_available - 1 WHERE id = ? AND seats_available > 0` :
   le décrément est conditionnel, donc atomique. `rowcount == 0` ⇒ `RideFull`.
3. `INSERT INTO bookings ...` : si l'index unique partiel est violé, la transaction est annulée et l'erreur devient
   `DuplicateBooking`. La place rendue par l'`UPDATE` est donc rendue en même temps.
4. `COMMIT`, puis lecture de la réservation créée.

Conséquences : aucun verrou applicatif, aucune lecture-puis-écriture, et la place ne peut pas être consommée par une
réservation refusée. Le cas UC5 (annulation) suivra le même schéma inverse (statut `CANCELLED` + incrément dans la même
transaction), avec la règle `départ - X` évaluée via le port `Clock`.

## 8. Modèle d'erreurs

Les erreurs métier sont déclarées dans `domain/errors.py` et **ne connaissent pas HTTP**. La correspondance est la
responsabilité de l'adaptateur (`api/http.py`) :

| Erreur | Origine | Code HTTP |
|---|---|---|
| `IdentityMissing` | application, levée par l'adaptateur du port `IdentityProvider` | 401 |
| `RideNotFound` | domaine | 404 |
| `RideFull` | domaine | 409 |
| `DuplicateBooking` | domaine (index unique) | 409 |
| `DriverCannotBook` | domaine | 400 |
| `InvalidRide` | domaine (champs manquants/incohérents) | 400 |
| erreur non prévue | infrastructure | 500 (journalisée, message générique) |

Le corps de réponse est `{"error": "<message>"}` — contrat conservé depuis UC3.

## 9. Correspondance UC → composants

| UC | Cas d'utilisation (application) | Ports sollicités |
|---|---|---|
| UC1 | `PublishRide` | `RideRepository`, `Clock` |
| UC2 | `SearchRides` | `RideRepository` (lecture filtrée) |
| **UC3** | **`BookSeat`** (implémenté) | `RideRepository`, `BookingRepository`, `NotificationPort`, `Clock` |
| UC5 | `CancelBooking` | `BookingRepository`, `NotificationPort`, `Clock` |
| UC6 | `RateRide` | `BookingRepository`, `RatingRepository`, `Clock` |
| UC7 | `GetRideDetails` | `RideRepository`, `ProfileRepository`, `RatingRepository` |
| UC8 | `ManageProfile` | `ProfileRepository` |
| UC9 | `UpdateOrDeleteRide` | `RideRepository`, `BookingRepository`, `NotificationPort` |
| UC10 | `ListMyRides` | `RideRepository`, `BookingRepository` |
| UC11 | `CompleteRide` | `RideRepository`, `NotificationPort` |
| UC12 | — (port utilisé par les autres UC) | `NotificationPort` |
| UC13 | — (port utilisé par la frontière HTTP) | `IdentityProvider` |

## 10. Écart avec l'implémentation actuelle

| Aujourd'hui | Cible | Raison |
|---|---|---|
| `app/models.py` | `app/domain/models.py` | le domaine se nomme, et porte ses invariants (statut typé, bornes des places) |
| `app/errors.py` (chaque erreur porte un `status` HTTP) | `app/domain/errors.py` + table dans `app/api/http.py` | le domaine ne doit pas connaître les codes HTTP |
| `app/services/booking_service.py` | `app/application/book_seat.py` | un cas d'utilisation par fichier, nommé par l'intention métier |
| `app/services/notification_service.py` | `app/infrastructure/notifications.py` | c'est un système externe, donc un adaptateur de port |
| `app/repositories/*.py` | `app/infrastructure/sqlite/*.py` | la persistance est un détail d'infrastructure |
| `app/db.py` (schéma + connexion mélangés) | `app/infrastructure/sqlite/{schema,database}.py` | séparer le DDL de la fabrique de connexions |
| `app/controllers/api.py` | `app/api/http.py` | nommer la couche par son rôle (adaptateur HTTP) |
| trous de câblage dans `create_app` | `app/__init__.py` + `app.container` | racine de composition explicite, testable ; `app.rides` devient `app.container.rides` |

Ce qui est **conservé** de l'implémentation actuelle, parce que le design le justifie : les entités `Ride`/`Booking`,
l'`UPDATE` conditionnel sous `BEGIN IMMEDIATE`, l'index unique partiel, le couple `db`/`sqlite3.Row`, et le contrat HTTP
(`POST /rides/<id>/bookings`, 201/400/401/404/409).

## 11. Décisions (ADR)

| ADR | Décision |
|---|---|
| [0001](adr/0001-couches-et-ports.md) | Quatre couches, dépendances vers l'intérieur, ports déclarés par le domaine |
| [0002](adr/0002-identite-externe.md) | Identité fournie par un système externe, derrière le port `IdentityProvider` |
| [0003](adr/0003-reservation-deux-etats.md) | Réservation à deux états, `ACTIVE` ou `CANCELLED`, sans étape de confirmation |
| [0004](adr/0004-concurrence-sqlite.md) | Concurrence traitée par `BEGIN IMMEDIATE` + `UPDATE` conditionnel + index unique partiel |
| [0005](adr/0005-persistance-sqlite.md) | SQLite sans ORM, DDL au démarrage, connexion par opération |
| [0006](adr/0006-modele-erreurs.md) | Erreurs de domaine sans HTTP, correspondance dans l'adaptateur |
| [0007](adr/0007-notification-au-mieux.md) | Notification au mieux, encadrée, sans remise en cause de la réservation |

## 12. Limites connues et points ouverts

- **Pas d'authentification applicative** : `X-User` est cru sur parole (décision du sujet, voir ADR 0002).
- **Notification au mieux** : un échec du système externe est journalisé, jamais propagé (ADR 0007 / UC12) ;
  vérifié par `test_notification_failure_keeps_the_booking`.
- **SQLite mono-fichier** : une instance par déploiement ; pas de réplication (ADR 0005).
- **Read models** : UC10 (voir mes trajets) lira probablement avec des requêtes dédiées plutôt que les dépôts d'écriture ;
  à trancher à l'itération 3.
- **Fin de trajet** : qui termine un trajet abandonné par le conducteur reste ouvert (UC11, voir la fiche UC).
