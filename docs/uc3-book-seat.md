# UC3 - Réserver une place

- **Acteur principal :** Passager  **Acteur secondaire :** Système de notification
- **Préconditions :** le trajet existe (l'utilisateur est identifié par l'en-tête `X-User`).
- **Postconditions (succès) :** une place est retenue ; une réservation `ACTIVE` existe ; le passager est notifié (UC12).

## Scénario nominal
1. Le passager envoie `POST /rides/{id}/bookings`.
2. Le système vérifie que le trajet existe.
3. Le système vérifie que le passager n'est pas le conducteur.
4. Le système décrémente atomiquement `seats_available` (uniquement si > 0) et crée une réservation `ACTIVE`.
5. Le système notifie le passager.
6. Le système répond `201 Created`.

## Scénarios alternatifs
| # | Condition | Résultat |
|---|---|---|
| A1 | Aucune identité | `401` |
| A2 | Trajet inconnu | `404` |
| A3 | Le passager est le conducteur | `400` |
| A4 | Plus aucune place (y compris si une requête concurrente prend la dernière) | `409`, rien n'est écrit |
| A5 | Le passager a déjà une réservation active sur ce trajet | `409`, place non consommée |

## Règles métier
- `seats_available` ne descend jamais sous 0 (`CHECK` SQL + `UPDATE` conditionnel).
- Une seule réservation active par passager et par trajet (index unique partiel).
- La décrémentation et l'insertion ont lieu dans une seule transaction.

## Tests
- Application (faux dépôts) : `internal/application/reserver_place_test.go`
- SQLite : `internal/infrastructure/sqlite/reservation_repository_test.go` — décrément, rollback sur doublon, clé étrangère et 8 appels concurrents sur 3 places (exactement 3 succès, 5 `409`)
- Intégration HTTP : `internal/api/reservations_test.go` — 3×`201` et 5×`409`, puis `GET /rides/{id}` confirme 0 place disponible
- Migration du schéma : `internal/infrastructure/sqlite/schema_test.go`
