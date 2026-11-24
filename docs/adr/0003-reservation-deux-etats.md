# ADR 0003 - Réservation à deux états : ACTIVE ou CANCELLED

- **Statut :** accepté
- **Date :** 2026-11-24

## Contexte

Une réservation retient une place sur un trajet. Aucune étape externe ne vient confirmer ou invalider une réservation
après sa création : tout ce qui doit être vérifié l'est au moment de la réservation (place disponible, pas de doublon,
passager différent du conducteur).

## Décision

- Une réservation a exactement deux états : `ACTIVE` (place retenue) et `CANCELLED` (place libérée). Aucun état
  intermédiaire, aucune transition en attente.
- `rides.price` est un **attribut descriptif** du trajet, jamais le support d'une transaction.
- L'annulation (UC5) passe la réservation à `CANCELLED` et libère la place dans la même transaction, sans autre effet.
- La notification annonce la réservation créée ; elle ne demande jamais au passager d'accomplir une action pour la valider.

## Conséquences

- **Positif :** un seul chemin d'état, donc des transitions faciles à tester et à lire ; une réservation est valide dès
  sa création, sans dépendance à un système externe ; aucune tâche de rattrapage si un partenaire externe est indisponible.
- **Négatif :** aucun contrôle différé n'est possible ; toute règle doit être évaluée au moment de la réservation.
- **Effet sur le modèle :** l'index unique partiel `(ride_id, passenger) WHERE status = 'ACTIVE'` porte directement la
  règle « une réservation active par passager et par trajet » (ADR 0004).

## Alternatives écartées

- **État d'attente jusqu'à une validation externe** : le périmètre ne contient aucune validation externe.
- **Réservation confirmée seulement après une action du conducteur** : contredit UC3, où la place est retenue immédiatement.
