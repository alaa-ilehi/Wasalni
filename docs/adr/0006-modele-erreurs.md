# ADR 0006 - Erreurs de domaine sans HTTP

- **Statut :** accepté
- **Date :** 2026-11-24

## Contexte

Dans le code actuel, chaque erreur métier porte son code HTTP sous forme d'attribut (`class RideFull(AppError): status = 409`).
Le domaine dépend donc du transport : une même règle ne pourrait pas être réutilisée par une CLI ou un consommateur non HTTP.

## Décision

- `app/domain/errors.py` déclare une hiérarchie `DomainError` (`RideNotFound`, `RideFull`, `DuplicateBooking`,
  `DriverCannotBook`, `InvalidRide`) **sans aucune notion de statut HTTP**.
- Les erreurs applicatives (identité absente) vivent dans `app/application/errors.py`.
- L'adaptateur HTTP porte l'unique table de correspondance erreur → code :
  `IdentityMissing` 401, `RideNotFound` 404, `RideFull` 409, `DuplicateBooking` 409, `DriverCannotBook` 400,
  `InvalidRide` 400.
- Toute autre exception est journalisée et rendue en `500` avec un message générique.
- Le corps de réponse reste `{"error": "<message>"}`.

## Conséquences

- **Positif :** le domaine est transportable ; la politique HTTP est lisible en un seul endroit ; une erreur ajoutée sans
  correspondance tombe en `500` — visible immédiatement en test plutôt qu'ignorée.
- **Négatif :** la table doit être maintenue à la main à chaque nouvelle erreur ; oublier une entrée change le code HTTP.
- **Garde-fou :** les tests d'intégration couvrent chaque code du contrat (201, 400, 401, 404, 409).

## Alternatives écartées

- **Exceptions Werkzeug (`abort`) depuis le service** : couple le métier à Flask.
- **Codes HTTP portés par le domaine** (statu quo) : simple, mais empêche la réutilisation et brouille la couche.
