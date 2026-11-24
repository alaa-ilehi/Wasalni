# ADR 0002 - Identité fournie par un système externe, derrière un port

- **Statut :** accepté
- **Date :** 2026-11-24

## Contexte

UC13 (s'authentifier) est délégué à un système d'authentification externe : le système ne stocke aucun mot de passe et
n'émet aucun jeton. Chaque appel arrive avec l'en-tête `X-User`, renseigné par le client.

L'identité est donc un acteur secondaire externe, au même titre que la notification : elle doit apparaître comme un port
dans l'architecture, sinon la frontière applicative dépend directement d'un détail de transport.

## Décision

- **L'identité est un port** : `IdentityProvider.current_actor() -> str`, déclaré dans `app/domain/ports.py`.
- Son adaptateur vit à la frontière HTTP : `app/api/identity.py`, classe `HeaderIdentityProvider`, qui lit `X-User`
  et lève `IdentityMissing` si l'en-tête est absent ou vide.
- L'adaptateur HTTP obtient l'identité par le port puis la passe **explicitement** au cas d'utilisation (`actor`).
  Aucun objet « utilisateur courant » ne circule dans l'application, et les cas d'utilisation restent testables avec une
  simple chaîne.
- Aucune authentification applicative : le système ne vérifie pas la preuve, il transporte une identité de confiance.

## Conséquences

- **Positif :** la couche `api` dépend d'un contrat du domaine et non d'un en-tête ; remplacer le transport (jeton signé,
  en-tête différent, passerelle amont) ne touche qu'un adaptateur ; `401` est produit à un seul endroit.
- **Négatif :** un port pour un seul adaptateur de production (la justification est ici le **système externe** derrière
  le port, pas le nombre d'implémentations) ; une indirection de plus à lire.
- **Limite assumée :** n'importe quel client connaissant un identifiant peut agir en son nom. C'est une hypothèse du
  sujet, pas un oubli ; elle est reprise dans les limites connues de [architecture.md](../architecture.md).

## Alternatives écartées

- **Lire `X-User` directement dans le routeur** (sans port) : couple la frontière à un détail de transport et duplique la
  règle du `401` à chaque nouvelle route.
- **Authentification interne** (mot de passe, session) : réintroduit un cas d'utilisation explicitement retiré du périmètre.
