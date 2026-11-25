# UC1 - Publier un trajet

- **Acteur principal :** Conducteur. **Acteur secondaire :** Système de notification (UC12, livré par webhook depuis la publication).
- **Préconditions :** l'utilisateur est identifié par un jeton de session (`Authorization: Bearer …`) ; il agit comme conducteur.
- **Postconditions (succès) :** le trajet existe avec `PUBLISHED`, autant de places disponibles que de places totales, et le délai d'annulation `X` stocké sur le trajet ; le conducteur en est le propriétaire.

Fiche issue du [cas d'utilisation UC1](use-case-elaboration.md#uc1---publier-un-trajet). Séquence : [uml/uc1-sequence.puml](uml/uc1-sequence.puml). Modèle de classes : [uml/class-model.puml](uml/class-model.puml).

## Scénario nominal

1. Le conducteur envoie `POST /rides` avec son jeton de session et un corps JSON : origine, destination, heure de départ, places, prix et `X` facultatif.
2. L'adaptateur HTTP lit l'identité, borne le corps à 10 Mio, refuse les champs inconnus et décode la saisie.
3. Le cas d'utilisation `PublierTrajet` construit le `Trajet` : places disponibles = places totales, statut `PUBLISHED`, délai par défaut de 2 h si `X` est absent.
4. Le domaine valide avec `ValiderPublication(maintenant)`, l'heure venant du port `Horloge` (aucune lecture directe de l'horloge système).
5. Le port `TrajetRepository` écrit via `Publier`, la base attribue l'identifiant et `created_at`, puis le service répond `201 Created` avec le trajet.

Requête :

```json
{"origin":"Lyon","destination":"Grenoble","departure_time":"2026-12-01T08:30:00Z",
 "seats_total":3,"price":12.5,"cancel_delay_hours":2}
```

Réponse `201` :

```json
{"id":"46d3…","driver_id":"bob","origin":"Lyon","destination":"Grenoble","departure_time":"2026-12-01T08:30:00Z",
 "seats_total":3,"seats_available":3,"price":12.5,"cancel_delay_hours":2,"status":"PUBLISHED",
 "created_at":"2026-11-24T21:35:47Z"}
```

## Scénarios alternatifs

| # | Condition | Résultat |
|---|---|---|
| A1 | jeton absent, inconnu ou expiré | `401` session absente, inconnue ou expirée |
| A2 | Origine ou destination vide, ou les deux égales (sans tenir compte de la casse) | `400`, motif en français |
| A3 | Places totales < 1, ou places disponibles hors de `[0, places_total]` | `400` |
| A4 | Prix négatif | `400` |
| A5 | Départ absent, passé, présent, ou hors format ISO 8601 | `400` |
| A6 | `X` négatif, ou supérieur ou égal au temps restant avant le départ | `400` |
| A7 | Corps JSON illisible, champ inconnu, ou corps au-delà de 10 Mio | `400` ou `413` |
| A8 | Erreur d'écriture du dépôt | `500`, cause journalisée |

Un refus de validation rend `{"error":"trajet invalide : l'origine et la destination doivent être différentes"}`. Aucun trajet n'est écrit quand une règle échoue.

## Règles métier

- Les clés du contrat sont en anglais (`origin`, `destination`, `departure_time`, `seats_total`, `price`, `cancel_delay_hours`) ; `departure` est accepté comme alias de `departure_time`. Le conducteur vient de la session, jamais du corps.
- Origine et destination sont débarrassées de leurs espaces, obligatoires et différentes ; la comparaison est insensible à la casse.
- Le départ est une date ISO 8601 strictement postérieure à l'instant fourni par l'horloge ; sans fuseau, il est lu en UTC.
- `X` (`cancel_delay_hours`) est stocké sur le trajet, exprimé en heures décimales, par défaut 2 h ; il doit être positif ou nul et strictement inférieur au temps restant avant le départ. `X = 0` autorise une annulation jusqu'au départ (UC5). L'échéance est `departure - X`.
- `seats_available` démarre à `seats_total` et le statut est `PUBLISHED` ; `COMPLETED` attend UC11.
- Le schéma SQLite porte les invariants (`CHECK` sur places, prix, délai et statut, DDL dans [architecture.md](architecture.md#9-modèle-de-données)).
- **Le décrément de `seats_available` est différé** (ADR 0004) : la publication écrit le compteur initial, UC3 ne le modifie pas encore. Le trajet publié est ensuite visible par la recherche (UC2) et lisible pour la réservation (UC3).

## Tests

Dérivés du scénario nominal et des règles ci-dessus :

- `internal/domain/trajet_test.go` — `ValiderPublication` accepte un trajet complet et refuse chaque règle, horloge fixe à la main.
- `internal/application/publier_trajet_test.go` — écriture `PUBLISHED` avec places au maximum, délai par défaut ou explicite, alias `departure`, date sans fuseau lue en UTC, refus sans écriture, erreur de dépôt propagée.
- `internal/infrastructure/sqlite/trajet_repository_test.go` — aller-retour `Publier`/`ParIdentifiant`, identifiant inconnu, places inchangées après réservation.
- `internal/api/trajets_test.go` — `401`, `400` par règle, `201` avec la forme exacte de la réponse, puis réservation réelle sur le trajet publié.

Commandes : `go test -race ./...` (également exécutée en CI, voir [ci.yml](../.github/workflows/ci.yml)).
