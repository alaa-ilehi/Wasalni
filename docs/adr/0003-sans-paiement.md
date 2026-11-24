# ADR 0003 - Périmètre sans paiement, réservations ACTIVE / CANCELLED

- **Statut :** accepté
- **Date :** 2026-11-24

## Contexte

Le paiement (ancien UC4) a été retiré du périmètre à l'itération d'élaboration : conducteur et passager règlent entre
eux, hors plateforme. Le code et les diagrammes portaient encore des états `PENDING` / `CONFIRMED` hérités de ce flux.

## Décision

- Aucun cas d'utilisation de paiement ; le paiement n'apparaît ni dans l'API, ni dans le modèle, ni dans les notifications.
- Une réservation a exactement deux états : `ACTIVE` et `CANCELLED`.
- `rides.price` reste un **attribut descriptif** du trajet (information affichée), jamais une transaction.
- L'annulation (UC5) libère une place sans mouvement financier.
- La notification n'envoie jamais d'invitation à payer : elle annonce la réservation.

## Conséquences

- **Positif :** un seul chemin d'état, plus simple à tester ; disparition du risque d'intégration d'un système de paiement ;
  une réservation est valide dès sa création, sans dépendance externe.
- **Négatif :** aucune garantie de solvabilité ni de remboursement côté plateforme — assumé par l'équipe.
- **Dette purgée :** l'identifiant UC4 n'est pas réutilisé ; aucune référence à l'ancien flux ne subsiste hors historique Git.

## Alternatives écartées

- **Paiement simulé** (service externe factice) : aurait porté un second risque technique, pour un UC retiré par décision d'équipe.
- **Garder `PENDING` avant confirmation manuelle** : sans paiement, plus rien ne confirmerait la réservation.
