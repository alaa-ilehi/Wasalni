# ADR 0004 - Concurrence : unicité en base aujourd'hui, décrément atomique différé

- **Statut :** partiellement appliqué — la garantie anti-doublon est en place ; le décrément atomique et son test de
  concurrence restent à écrire dès que la table `rides` existera (#5, #10)
- **Date :** 2026-11-24 (révision)

## Contexte

La réservation concurrente est le risque technique n°1 du projet : plusieurs passagers visent la dernière place au même
instant. UC3 est livré dans un périmètre réduit : la table `rides` appartient à UC1/UC2, donc le service écrit des
réservations sans tenir de compteur de places. Deux courses différentes coexistent alors :

1. **le doublon** — le même passager réserve deux fois le même trajet ;
2. **la dernière place** — deux passagers différents la prennent en même temps.

La première est arbitrable dès maintenant par la base ; la seconde exige une colonne `seats_available` que le service ne
possède pas.

## Décision

**Appliqué aujourd'hui :**

1. une seule connexion (`SetMaxOpenConns(1)`, `SetMaxIdleConns(1)`) et `PRAGMA busy_timeout = 5000` ;
2. transactions ouvertes en mode immédiat (option `_txlock=immediate` du pilote, posée dans `sqlite.Ouvrir`) : le verrou
   d'écriture est pris à l'ouverture, sans escalade lecture → écriture ;
3. `INSERT INTO bookings` sous l'index unique partiel `(ride_id, passenger) WHERE status = 'ACTIVE'` ; la violation de
   contrainte (code 2067) est traduite en `domain.ErrReservationDejaActive`. La course est arbitrée par la base, jamais
   par une lecture préalable ;
4. aucune boucle de reprise, aucun verrou applicatif.

**Différé, avec sa raison :**

- Sans table `rides`, il n'y a pas de `seats_available` : le refus « trajet complet » vient de la vue `Trajet` lue par
  `ReserverPlace`, et ne protège donc pas la dernière place. Deux requêtes simultanées sur la dernière place peuvent
  aboutir aujourd'hui.
- Le test de concurrence ne peut pas être écrit sans compteur : il n'y aurait rien à départager.
- À la livraison de `rides` (#5, #10), la suite est connue et courte : `UPDATE rides SET seats_available =
  seats_available - 1 WHERE id = ? AND seats_available > 0` dans la **même** transaction immédiate que l'insertion,
  zéro ligne modifiée ⇒ `domain.ErrTrajetComplet`, puis le test à trois places et trois succès.

## Conséquences

- **Aujourd'hui :** un passager ne peut pas obtenir deux réservations actives sur le même trajet, même sous appels
  simultanés ; la dernière place reste en revanche exposée à la surréservation.
- Le port `domain.ReservationRepository` porte déjà le contrat (« échoue avec `ErrReservationDejaActive` »), donc
  l'adaptateur futur n'obligera pas à modifier le cas d'usage.
- **Négatif assumé :** deux sources de vérité pour « complet » (la vue `Trajet`) et « doublon » (l'index) tant que
  `rides` n'existe pas. C'est la dette suivie par l'issue de suivi de #12.
- **Preuve actuelle :** `internal/infrastructure/sqlite/reservation_repository_test.go` (doublon refusé, aucune seconde
  ligne écrite ; index partiel qui laisse passer un autre passager) et `internal/api/reservations_test.go` (`409`).

## Alternatives écartées

- **Verrou applicatif (`sync.Mutex`)** : ne protège pas deux processus et mêle le métier au déploiement.
- **`SELECT` puis `INSERT`** : fenêtre de course entre la lecture et l'écriture, exactement ce que l'index supprime.
- **Verrou optimiste avec numéro de version** : correct mais plus complexe pour un seul point d'écriture.
- **Attendre #5/#10 avant d'écrire UC3** : la livraison du cas d'utilisation et de ses tests ne dépend pas du compteur ;
  seule la garantie sur la dernière place en dépend, et elle est explicitement différée.
- **Supprimer l'ADR avec le code retiré** : la course sur la dernière place reste le risque du projet, et la fiche
  [UC3](../uc3-book-seat.md) la décrit toujours.
