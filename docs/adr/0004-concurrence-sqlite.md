# ADR 0004 - Concurrence traitée dans l'adaptateur par transaction immédiate

- **Statut :** accepté
- **Date :** 2026-11-24

## Contexte

La réservation concurrente est le risque technique n°1 du projet : plusieurs passagers visent la dernière place au même
instant. Le contrat HTTP doit rester sans surréservation, et le test `tests/test_concurrency.py` envoie 15 requêtes
simultanées sur 3 places.

## Décision

Toute la garantie tient dans le port `BookingRepository.book_seat`, implémenté en SQLite par :

1. `BEGIN IMMEDIATE` — verrou d'écriture pris dès l'ouverture, pas d'escalade lecture → écriture sous charge ;
2. `UPDATE rides SET seats_available = seats_available - 1 WHERE id = ? AND seats_available > 0` — décrément
   conditionnel atomique ; `rowcount == 0` ⇒ `RideFull` ;
3. `INSERT INTO bookings` sous index unique partiel `(ride_id, passenger) WHERE status = 'ACTIVE'` ;
   violation ⇒ `ROLLBACK` + `DuplicateBooking` ;
4. `COMMIT`, puis relecture de la réservation.

Aucun verrou applicatif, aucune lecture-puis-écriture, aucune boucle de reprise dans le domaine.

## Conséquences

- **Positif :** une place ne peut être ni vendue deux fois, ni consommée par une réservation refusée ; la propriété est
  locale à une fonction, donc relisible et testable ; l'`UPDATE` conditionnel est portable vers PostgreSQL.
- **Négatif :** les écritures sont sérialisées par le verrou SQLite (acceptable : un seul fichier, faible volume) ;
  il faut un `busy_timeout` pour absorber les attentes sous concurrence.
- **Effet de bord souhaitable :** le test de concurrence reste la preuve d'acceptation de cette décision ; sa suppression
  invaliderait l'ADR.

## Alternatives écartées

- **Verrou Python (`threading.Lock`)** : ne protège pas deux processus, et mélange le métier avec le déploiement.
- **`SELECT` puis `UPDATE`** : fenêtre de course entre lecture et écriture.
- **Verrou optimiste avec numéro de version** : correct mais plus complexe pour un seul point d'écriture.
