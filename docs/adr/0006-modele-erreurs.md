# ADR 0006 - Erreurs de domaine sans HTTP, correspondance unique dans l'adaptateur

- **Statut :** accepté (révisé lors du portage en Go)
- **Date :** 2026-11-24

## Contexte

Une erreur métier qui porte son code HTTP oblige le domaine à connaître le transport : la même règle ne pourrait pas être
réutilisée par une CLI ou un consommateur non HTTP.

En Go, les erreurs sont des valeurs : la forme naturelle est une sentinelle comparable par `errors.Is`, et non une
hiérarchie de classes portant un attribut de statut.

## Décision

- Le domaine déclare ses erreurs comme sentinelles, sans aucune notion de statut HTTP (`internal/domain/profil.go`) :
  `ErrNomRequis`, `ErrNomInvalide`, `ErrAgeInvalide`, `ErrSexeInvalide`, `ErrProfilIntrouvable`. Elles sont enveloppables
  (`fmt.Errorf("… : %w", err)`) et restent reconnaissables par `errors.Is`.
- L'adaptateur SQLite traduit `sql.ErrNoRows` en `domain.ErrProfilIntrouvable` : le domaine ne connaît pas `database/sql`.
- Le pipeline photo déclare ses propres sentinelles, `ErrImageIllisible` et `ErrImageTropLourde`
  (`internal/infrastructure/photos`) ; le cas d'usage les enveloppe en `application.ErrPhotoInvalide`, seule erreur que
  l'adaptateur HTTP connaisse pour une photo refusée.
- L'identité absente est une erreur de l'adaptateur HTTP, `api.ErrIdentiteManquante` : elle n'existe pas dans l'application
  (ADR 0002).
- L'adaptateur HTTP porte l'**unique** table de correspondance erreur → code, la fonction `statutHTTP` de
  `internal/api/http.go` : `401` identité absente, `404` profil introuvable, `400` nom, âge, sexe ou image refusés.
- Toute autre erreur est journalisée (`log/slog`) et rendue en `500` avec le message générique « erreur interne » ; les
  erreurs attendues sont rendues avec leur message métier.
- Le corps de réponse reste `{"error": "<message>"}`. Un corps de requête au-delà de 10 Mio est refusé par
  `http.MaxBytesReader` en `413` avant d'atteindre la table.

## Conséquences

- **Positif :** le domaine est transportable ; la politique HTTP est lisible en une fonction ; une erreur ajoutée sans
  correspondance tombe en `500`, visible immédiatement en test plutôt qu'ignorée ; les messages métier restent en français
  et les codes en un seul endroit.
- **Négatif :** la table doit être maintenue à la main à chaque nouvelle erreur ; oublier une entrée change le code HTTP.
- **Garde-fou :** les tests d'intégration HTTP couvrent chaque code du contrat (`200`, `201`, `400`, `401`, `404`, `413`).

## Alternatives écartées

- **Une interface `interface{ HTTPStatus() int }` portée par les erreurs** : le domaine dépendrait de `net/http`, et
  l'enveloppement d'erreurs devrait propager le statut explicitement.
- **Erreurs typées avec un champ `status`** (forme de l'implémentation Python retirée) : simple, mais empêche la
  réutilisation et brouille la couche.
- **`http.Error` dispersé dans les gestionnaires** : duplique la politique et casse l'unicité de la table.
