# ADR 0004 - Concurrence : unicité et décrément atomique en base

- **Statut :** appliqué — doublon et dernière place sont arbitrés par SQLite dans la même transaction
- **Date :** 2026-11-24 (révision)

## Contexte

La réservation concurrente est le risque technique n°1 du projet : plusieurs passagers visent la dernière place au même
instant. Deux courses coexistent :

1. **le doublon** — le même passager réserve deux fois le même trajet ;
2. **la dernière place** — deux passagers différents la prennent en même temps.

La lecture de `rides.seats_available` faite par `ReserverPlace` ne suffit pas : entre la lecture et l'écriture, un autre
appel peut prendre la place. L'arbitrage doit donc se faire dans la base, au moment de l'écriture.

## Décision

1. une seule connexion (`SetMaxOpenConns(1)`, `SetMaxIdleConns(1)`) et `PRAGMA busy_timeout = 5000` ;
2. transactions ouvertes en mode immédiat (option `_txlock=immediate` du pilote, posée dans `sqlite.Ouvrir`) : le verrou
   d'écriture est pris à l'ouverture, sans escalade lecture → écriture ;
3. `INSERT INTO bookings` sous l'index unique partiel `(ride_id, passenger) WHERE status = 'ACTIVE'` ; la violation de
   contrainte (code 2067) est traduite en `domain.ErrReservationDejaActive` ;
4. avant l'insertion, `UPDATE rides SET seats_available = seats_available - 1 WHERE id = ? AND seats_available > 0` ;
   zéro ligne modifiée ⇒ `domain.ErrTrajetComplet` ;
5. les deux écritures sont dans la **même** transaction : un refus annule le décrément, un commit réussi rend le
   compteur et la réservation cohérents ;
6. `bookings.ride_id` référence `rides(id)` ; une migration idempotente ajoute la contrainte sur les bases créées avant
   cette décision, sans perdre les réservations existantes ;
7. aucune boucle de reprise, aucun verrou applicatif.

Le port `domain.ReservationRepository`, le cas d'usage `ReserverPlace` et le contrat HTTP ne changent pas.

## Conséquences

- Un passager ne peut pas obtenir deux réservations actives sur le même trajet, même sous appels simultanés.
- La dernière place est attribuée à exactement un passager ; les autres reçoivent `409` sans écriture.
- Le contrôle rapide fait par `ReserverPlace` reste utile pour refuser tôt un trajet déjà complet, mais il n'est plus la
  source de vérité.
- **Preuves :** `reservation_repository_test.go` (décrement, rollback sur doublon, 8 appels concurrents sur 3 places),
  `trajet_repository_test.go` (compteur lu après réservation), `schema_test.go` (migration de la clé étrangère) et
  `internal/api/reservations_test.go` (3×`201`, 5×`409`, puis 0 place via UC7).

## Alternatives écartées

- **Verrou applicatif (`sync.Mutex`)** : ne protège pas deux processus et mêle le métier au déploiement.
- **`SELECT` puis `INSERT`** : fenêtre de course entre la lecture et l'écriture, exactement ce que la base arbitre ici.
- **Verrou optimiste avec numéro de version** : correct mais plus complexe pour un seul point d'écriture.
- **Décrément sans transaction** : laisserait un ride sans place avec aucune réservation, ou l'inverse, en cas d'erreur.
- **Supprimer l'ADR avec le code retiré** : la course reste le risque du projet, et la fiche [UC3](../uc3-book-seat.md)
  la décrit.
