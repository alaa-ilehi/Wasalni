# ADR 0004 - Concurrence traitée dans l'adaptateur par transaction immédiate

- **Statut :** en veille — la stratégie est conservée pour la réimplémentation de UC3, aucun code ne l'applique aujourd'hui
- **Date :** 2026-11-24

## Contexte

La réservation concurrente est le risque technique n°1 du projet : plusieurs passagers visent la dernière place au même
instant. Le contrat devra rester sans surréservation.

Le service actuel ne réserve aucune place : il ne gère que les profils, où deux appelants n'écrivent jamais la même ligne
(l'identifiant est la clé primaire, et l'écriture est un upsert transactionnel). Le besoin de cette décision n'existe donc
pas encore, mais il réapparaîtra intact dès que UC3 reviendra.

La base est ouverte avec une seule connexion (`SetMaxOpenConns(1)`, `SetMaxIdleConns(1)`) et `PRAGMA busy_timeout = 5000`,
ce qui sérialise déjà les écritures du service actuel.

## Décision

La stratégie est conservée telle quelle, pour la future implémentation de UC3, dans le port de réservation de
l'adaptateur SQLite :

1. transaction immédiate — le verrou d'écriture est pris dès l'ouverture, pas d'escalade lecture → écriture sous charge ;
2. `UPDATE rides SET seats_available = seats_available - 1 WHERE id = ? AND seats_available > 0` — décrément
   conditionnel atomique ; zéro ligne modifiée ⇒ trajet complet ;
3. `INSERT INTO bookings` sous index unique partiel `(ride_id, passenger) WHERE status = 'ACTIVE'` ; violation ⇒
   annulation de la transaction et erreur de doublon ;
4. validation, puis relecture de la réservation.

Aucun verrou applicatif, aucune lecture-puis-écriture, aucune boucle de reprise dans le domaine.

## Conséquences

- **Aujourd'hui :** aucune. `internal/infrastructure/sqlite` n'implémente que `ProfilRepository`, dont l'upsert tient dans
  une transaction sans concurrence sur une même ligne.
- **À la réimplémentation de UC3 :** une place ne pourra être ni vendue deux fois, ni consommée par une réservation
  refusée ; la propriété sera locale à une fonction, donc relisible et testable ; le décrément conditionnel est portable
  vers PostgreSQL.
- **Négatif à venir :** les écritures sont sérialisées par le verrou SQLite (acceptable : un seul fichier, faible volume) ;
  il faudra un `busy_timeout` pour absorber les attentes sous concurrence — il est déjà posé.
- **Preuve d'acceptation à réécrire :** le test de concurrence qui accompagnait l'implémentation Python a disparu avec
  elle. Il devra être réécrit en Go (plusieurs appels simultanés pour trois places, exactement trois succès) : sans lui,
  la décision reste une intention.

## Alternatives écartées

- **Verrou applicatif (`sync.Mutex`)** : ne protège pas deux processus, et mélange le métier avec le déploiement.
- **`SELECT` puis `UPDATE`** : fenêtre de course entre lecture et écriture.
- **Verrou optimiste avec numéro de version** : correct mais plus complexe pour un seul point d'écriture.
- **Supprimer l'ADR avec le code retiré** : la course sur la dernière place reste le risque du projet, et la fiche
  [UC3](../uc3-book-seat.md) la décrit toujours.
