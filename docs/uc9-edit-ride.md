# UC9 — Modifier ou supprimer un trajet publié

- **Acteur :** conducteur propriétaire. **Précondition :** trajet publié et non supprimé.
- **Modifier :** `PUT /rides/{id}` avec `X-User` et les champs éditables du trajet.
- **Supprimer :** `DELETE /rides/{id}` avec `X-User`.
- **Postconditions :** les modifications et le recalcul des places sont atomiques. Une suppression logique conserve les références historiques, annule dans la même transaction les réservations actives et retire le trajet des recherches et consultations.

## Règles

Le conducteur doit être propriétaire du trajet. Un trajet inconnu ou supprimé renvoie `404`, un autre propriétaire `403`, un trajet terminé `409`. Une modification dont `seats_total` est inférieur au nombre de réservations actives renvoie `409`; les places disponibles deviennent `seats_total - réservations actives`. Les champs sont validés selon les règles de publication, notamment la date et le délai d'annulation. Modifier `cancel_delay_hours` met donc à jour le délai applicable aux réservations conservées.

La suppression garde la ligne `rides` et les lignes `bookings` pour préserver les clés étrangères et l'historique. Elle pose `deleted_at` et annule les réservations actives dans une transaction. L'insertion de nouvelles réservations est conditionnée à un trajet publié et non supprimé.

Les notifications de modification/suppression sont journalisées après commit au mieux. Leur livraison effective est externe et relève d'UC12 (ADR 0007).

## Séquence nominale

1. Le conducteur envoie la modification complète ou demande la suppression.
2. L'API authentifie `X-User`; le cas d'utilisation et le dépôt vérifient le propriétaire et l'état.
3. Pour une modification, le dépôt compte les réservations actives, valide la capacité et écrit les champs et les places disponibles dans une transaction.
4. Pour une suppression, le dépôt annule les réservations actives et marque le trajet supprimé dans une transaction.
5. L'API répond `200` avec le trajet modifié ou `204` après suppression.

Séquence : [uc9-sequence.puml](uml/uc9-sequence.puml). Modèle de classes : [class-model.puml](uml/class-model.puml).
