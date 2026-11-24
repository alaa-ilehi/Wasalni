# ADR 0010 - Évaluations en table dédiée, moyenne calculée à la lecture

- **Statut :** appliqué
- **Date :** 2026-11-24

## Contexte

UC6 enregistre des jugements croisés après un trajet : le conducteur note chaque passager, chaque passager note le
conducteur. UC7 doit afficher la moyenne du conducteur. UC11 n'est pas encore livré, mais `rides.status` connaît déjà
`COMPLETED`.

Deux modélisations sont possibles : stocker une moyenne et un compteur sur `users`, ou stocker les évaluations elles-mêmes
et calculer la moyenne à la lecture. Le volume est faible (quelques notes par profil) et la justesse importe plus que le
coût d'un `AVG`.

## Décision

1. Une table `ratings` : `ride_id` référence `rides(id)`, `evaluator`, `target`, `score` entre 1 et 5, `comment`
   facultatif, `created_at`.
2. Un index unique `(ride_id, evaluator, target)` porte la règle « une évaluation par couple et par trajet » : le doublon
   est arbitrée par la base, sans lecture préalable, et traduit en `domain.ErrEvaluationDejaFaite`.
3. Aucune méthode `PUT` ou `DELETE` : une évaluation est immuable.
4. `EvaluationRepository.Moyenne(identifiant)` calcule `AVG(score)` et `COUNT(*)` en SQL, arrondit à une décimale et
   rend zéro quand il n'y a aucune note.
5. `ConsulterTrajet` injecte cette moyenne dans `driver.rating` (UC7). `null` signifie « aucune évaluation reçue », pas
   « note nulle ». Le contrat `GET /profile` ne change pas.
6. Un refus d'évaluation distingue le trajet non terminé (`409`), le non-participant (`403`) et le doublon (`409`). Les
   erreurs métier restent en français et sans code HTTP dans le domaine (ADR 0006).

## Conséquences

- **Positif :** aucune colonne redondante à maintenir sur `users` ; l'ajout ou la suppression d'une évaluation ne peut pas
  désynchroniser une moyenne ; le doublon est sûr sous appels simultanés.
- **Négatif :** chaque `GET /rides/{id}` porte un `AVG` supplémentaire ; acceptable à ce volume, avec une seule connexion
  SQLite (ADR 0005).
- **Dépendance :** tant que UC11 n'existe pas, l'endpoint est inatteignable en production. Les tests posent directement
  `status = 'COMPLETED'` ; cette dette est documentée dans `docs/architecture.md` §15.
- **Participation :** en l'absence d'UC5, la présence d'une ligne `bookings` — quel que soit son état — atteste de la
  participation. UC5 précisera si une réservation annulée doit exclure son passager des évaluations.

## Alternatives écartées

- **Moyenne et compteur sur `users`** : deux écritures à synchroniser avec l'évaluation, et aucune trace des notes
  individuelles pour les tester.
- **Évaluations stockées dans le profil** : mélangerait UC8 et UC6 et rendrait la mise à jour concurrente plus difficile.
- **`rating: 0` par défaut** : donnerait l'impression d'une mauvaise note ; `null` distingue l'absence d'évaluation.
- **Autoriser les passagers à s'évaluer entre eux** : la fiche UC6 parle des « deux parties » du trajet, pas de paires de
  passagers.
