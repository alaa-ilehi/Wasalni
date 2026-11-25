# ADR 0004 - Concurrence : unicité en base et décrément atomique des places

- **Statut :** appliqué — l'index unique partiel arbitre le doublon, le décrément conditionnel dans la transaction
  immédiate arbitre la dernière place, et `bookings.ride_id` référence `rides(id)`
- **Date :** 2026-11-24 (révision)

## Contexte

La réservation concurrente est le risque technique n°1 du projet : plusieurs passagers visent la dernière place au même
instant. Deux courses différentes coexistent :

1. **le doublon** — le même passager réserve deux fois le même trajet ;
2. **la dernière place** — deux passagers différents la prennent en même temps.

La première est arbitrée par la base ; la seconde exige de décrémenter `rides.seats_available` dans la même transaction
que l'insertion. Les deux garanties sont désormais dans l'adaptateur SQLite, et le test de concurrence les couvre.

## Décision

**Appliqué :**

1. une seule connexion (`SetMaxOpenConns(1)`, `SetMaxIdleConns(1)`) et `PRAGMA busy_timeout = 5000` ;
2. transactions ouvertes en mode immédiat (option `_txlock=immediate` du pilote, posée dans `sqlite.Ouvrir`) : le verrou
   d'écriture est pris à l'ouverture, sans escalade lecture → écriture ;
3. `UPDATE rides SET seats_available = seats_available - 1 WHERE id = ? AND seats_available > 0` **avant** l'insertion,
   dans cette même transaction : zéro ligne modifiée ⇒ transaction annulée et `domain.ErrTrajetComplet`. La place est
   donc consommée par la base, pas par une lecture préalable ;
4. `INSERT INTO bookings` sous l'index unique partiel `(ride_id, passenger) WHERE status = 'ACTIVE'` ; la violation de
   contrainte (code 2067) est traduite en `domain.ErrReservationDejaActive`. L'annulation qui suit rend la place
   décrémentée à l'étape 3 : un doublon refusé ne coûte pas de place ;
5. `bookings.ride_id REFERENCES rides(id)` : une réservation ne peut pas viser un trajet inexistant ;
6. aucune boucle de reprise, aucun verrou applicatif.

L'ordre « décrément puis insertion » est délibéré : le refus « trajet complet » ne demande qu'un `UPDATE`, et le cas
défavorable (doublon) reste correct parce que la transaction est annulée en bloc.

## Conséquences

- Un passager ne peut pas obtenir deux réservations actives sur le même trajet, même sous appels simultanés, et aucun
  trajet ne peut être surréservé : la place est consommée par un `UPDATE` conditionnel unique, sérialisé par la
  transaction immédiate.
- Le port `domain.ReservationRepository` n'a pas changé : le contrat « échoue avec `ErrReservationDejaActive` » était
  déjà écrit, et `ErrTrajetComplet` existait dans le domaine. Aucune signature de cas d'usage ni de route n'a bougé.
- **Coût assumé :** une place est décrémentée avant l'insertion ; si l'insertion échoue pour une autre raison, la place
  n'est rendue que par l'annulation de la transaction, ce qui suppose que l'adaptateur n'oublie jamais son `defer`
  `Rollback` (en place).
- La suppression d'un trajet (UC9) annule ses réservations actives dans la même transaction **et rend leurs places**
  (`seats_available = MIN(seats_available + annulées, seats_total)`), pour que la ligne conservée par la suppression
  logique respecte encore l'invariant « places + réservations actives = places totales ».
- La clé étrangère a été ajoutée par **reconstruction de table** (`bookings` → `bookings_ancienne` → `bookings`), SQLite
  ne sachant pas modifier une table existante. La copie est stricte : une réservation orpheline fait échouer le
  démarrage plutôt que d'être perdue en silence.
- **Preuve :** `internal/infrastructure/sqlite/reservation_repository_test.go` (place consommée, dernière place refusée,
  doublon sans perte de place, huit réservations concurrentes pour trois places donnant exactement trois succès, clé
  étrangère qui refuse une réservation orpheline, migration qui conserve les réservations),
  `internal/api/trajets_test.go` (`201`, `201`, `seats_available: 0`, puis `409`) et le smoke sur le binaire publié.

## Alternatives écartées

- **Verrou applicatif (`sync.Mutex`)** : ne protège pas deux processus et mêle le métier au déploiement.
- **`SELECT` puis `INSERT`** : fenêtre de course entre la lecture et l'écriture, exactement ce que le `UPDATE`
  conditionnel supprime.
- **Insérer puis décrémenter** : une réservation écrite avant d'être sûre d'avoir une place obligerait à compenser par
  une écriture inverse, hors transaction.
- **Verrou optimiste avec numéro de version** : correct mais plus complexe pour un seul point d'écriture.
- **Mettre la clé étrangère dans une migration versionnée** : le schéma reste rejouable sans table de version ; le
  contrôle `PRAGMA foreign_key_list` suffit à ne reconstruire qu'une base ancienne.
- **Attendre #5/#10 avant d'écrire UC3** : la livraison du cas d'utilisation et de ses tests ne dépendait pas du
  compteur ; cette étape est maintenant faite, l'écart est refermé.
- **Supprimer l'ADR avec le code retiré** : la course sur la dernière place reste le risque du projet, et la fiche
  [UC3](../uc3-book-seat.md) la décrit toujours.
