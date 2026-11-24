# UC6 - S'évaluer mutuellement

- **Acteur :** Utilisateur, dans les deux rôles (conducteur et passager).
- **Préconditions :** le trajet est terminé (`COMPLETED`, posé par UC11) et l'évaluateur y a participé.
- **Postconditions (succès) :** une évaluation est enregistrée pour l'autre partie ; la moyenne du conducteur est lisible dans UC7.

## Scénario nominal

1. L'utilisateur ouvre un trajet terminé depuis UC10.
2. Il envoie `POST /rides/{id}/ratings` avec `X-User`, un `score` de 1 à 5, la `target` et un `comment` facultatif.
3. Le système vérifie le trajet terminé et la participation des deux parties.
4. Le système enregistre l'évaluation ; l'index unique refuse un second dépôt pour le même couple.
5. Le système répond `201 Created`.

## Scénarios alternatifs

| # | Condition | Résultat |
|---|---|---|
| A1 | Aucune identité | `401` |
| A2 | Note hors 1–5, cible absente, commentaire > 500 caractères | `400` |
| A3 | Évaluateur ou cible non participant | `403` |
| A4 | Trajet inconnu | `404` |
| A5 | Trajet non terminé (`PUBLISHED`) | `409` |
| A6 | Couple `(trajet, évaluateur, cible)` déjà évalué | `409` |

## Règles métier

- Une évaluation par couple et par trajet ; conducteur et passagers évaluent indépendamment.
- Un passager évalue le conducteur ; le conducteur évalue un passager. Un passager n'évalue pas un autre passager.
- Aucune modification après dépôt : ni `PUT` ni `DELETE` sur une évaluation.
- La moyenne est calculée à la lecture et arrondie à une décimale (ADR 0010) ; sans note, UC7 renvoie `rating: null`.

## Dépendance

UC11 n'est pas livré : aucun endpoint ne pose encore `COMPLETED`. La route refuse donc tout trajet publié en production ;
les tests insèrent directement le statut `COMPLETED` pour couvrir UC6 isolément.

## Tests

- Domaine : `internal/domain/evaluation_test.go` — bornes de la note, commentaire, auto-évaluation.
- Application : `internal/application/evaluer_participant_test.go` — évaluations croisées, participants, doublon.
- SQLite : `internal/infrastructure/sqlite/evaluation_repository_test.go` — unicité, moyenne, arrondi.
- HTTP : `internal/api/ratings_test.go` — codes de refus, évaluations croisées, moyenne lue dans `GET /rides/{id}`.
