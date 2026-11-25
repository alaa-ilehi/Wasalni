# UC5 - Annuler une réservation

- **Acteur principal :** Passager. **Acteur secondaire :** Système de notification (UC12).
- **Préconditions :** le passager est identifié par `X-User` et possède une réservation `ACTIVE`.
- **Postconditions (succès) :** la réservation devient `CANCELLED` et exactement une place est rendue au trajet.

Séquence : [uml/uc5-sequence.puml](uml/uc5-sequence.puml). Modèle de classes : [uml/class-model.puml](uml/class-model.puml).

## Scénario nominal

1. Le passager envoie `DELETE /bookings/{id}` avec `X-User`.
2. Le cas d'utilisation lit l'heure via `Horloge` et demande au dépôt d'annuler la réservation de cet acteur.
3. Le dépôt vérifie propriété, état et échéance `now < departure - X` dans une transaction immédiate.
4. Dans cette transaction, le dépôt passe la réservation à `CANCELLED`, incrémente `seats_available` une seule fois et valide les deux écritures ensemble.
5. Après validation, le cas d'utilisation journalise l'événement au mieux et répond `204 No Content`. La livraison aux
   deux parties dépend de l'adaptateur externe UC12, non présent dans cette implémentation.

## Scénarios alternatifs

| # | Condition | Résultat |
|---|---|---|
| A1 | `X-User` absent ou vide | `401`, aucune écriture |
| A2 | Réservation inconnue ou appartenant à un autre passager | `404`, aucune écriture |
| A3 | Réservation déjà annulée | `409`, aucune place supplémentaire |
| A4 | Échéance atteinte ou dépassée (`now >= departure - X`) | `409`, aucune écriture |
| A5 | Journalisation indisponible | annulation conservée ; réponse inchangée |
| A6 | Erreur SQLite pendant la transaction | `500`, réservation et compteur reviennent à leur état initial |

## Règles métier

- `X` est lu sur le trajet et n'est jamais fourni par le passager.
- Propriété, statut `ACTIVE` et échéance sont vérifiés avant la transition.
- La mise à jour de `bookings.status` et le retour de la place sont atomiques.
- Les annulations concurrentes sont sérialisées ; seule la première peut rendre une place.
- L'événement part après le commit et ne défait jamais l'annulation (ADR 0007). La livraison externe n'est pas faite.

## Tests

- Domaine : propriété, état, trajet correspondant et frontière exacte de l'échéance.
- Application : identité requise, passage de l'heure au dépôt, notification après succès et échec de notification sans rollback.
- SQLite : retour de place, propriétaire, date limite, répétition, et concurrence sur la même réservation.
- HTTP/SQLite : `204`, `401`, `404`, `409`, état final et compteur de places.
