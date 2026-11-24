# ADR 0002 - Identité fournie par un système externe

- **Statut :** accepté
- **Date :** 2026-11-24

## Contexte

UC13 (s'authentifier) est délégué à un système d'authentification externe : le système ne stocke aucun mot de passe et
n'émet aucun jeton. Chaque appel arrive avec l'en-tête `X-User`, renseigné par le client.

## Décision

- `X-User` est l'identité de confiance du système.
- L'extraction de l'en-tête est la responsabilité de l'adaptateur HTTP (`app/api/http.py`), qui la transmet au cas
  d'utilisation sous forme d'argument `actor`.
- **Aucun port d'identité** n'est créé : il n'y aurait qu'une implémentation, et le domaine n'a pas besoin de connaître
  la notion d'utilisateur courant.
- En-tête absent ou vide ⇒ `IdentityMissing` ⇒ `401`.

## Conséquences

- **Positif :** aucun état de session, aucun secret stocké, tests triviaux ; le domaine reçoit une chaîne et la traite
  comme un identifiant opaque.
- **Négatif :** n'importe quel client connaissant un identifiant peut agir en son nom. C'est une hypothèse du sujet, pas
  un oubli ; elle est notée dans les limites connues de [architecture.md](../architecture.md).
- **Si l'hypothèse change** (jeton signé à vérifier), un port `IdentityProvider` devient justifié : l'application
  vérifierait la preuve au lieu de recevoir un `actor` nu.

## Alternatives écartées

- **Authentification interne** (mot de passe, session) : réintroduit un UC explicitement retiré du périmètre.
- **Port d'identité** : dépendance inversée sans second implémenteur — abstraction spéculative.
