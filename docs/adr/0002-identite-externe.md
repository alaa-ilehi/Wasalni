# ADR 0002 - Identité fournie par un système externe, lue dans l'adaptateur HTTP

- **Statut :** révisé par [ADR 0011](0011-authentification-sessions.md) — la lecture de l'identité reste dans l'adaptateur
  HTTP, mais la preuve d'identité est désormais une session (UC13)
- **Date :** 2026-11-24

## Révision (2026-11-25)

Les décisions de cadrage restent valables : l'identité est lue dans l'adaptateur HTTP, elle n'est pas un port du domaine,
elle est passée explicitement aux cas d'utilisation sous forme de chaîne, et le `401` reste produit par la table
`statutHTTP`. Ce qui change : **« aucune authentification applicative » n'est plus vrai**. L'en-tête `X-User` croyait le
client sur parole ; l'ADR 0011 le remplace par un jeton de session (`Authorization: Bearer …`) vérifié en base, avec
mots de passe hachés (PBKDF2-HMAC-SHA256, sel par compte) et trois routes `/auth/*`. Toute mention ci-dessous d'un
en-tête de confiance est donc historique.

## Contexte

UC13 (s'authentifier) est délégué à un système d'authentification externe : le service ne stocke aucun mot de passe et
n'émet aucun jeton. Chaque appel arrive avec l'en-tête `X-User`, renseigné par le client.

L'identité doit rester externe, mais elle traverse la frontière HTTP à chaque requête : il faut décider où elle est lue
et comment elle parvient au cas d'utilisation.

## Décision

- **L'identité est lue dans l'adaptateur HTTP** : `internal/api/identite.go` expose la constante `EnTeteIdentite`
  (`X-User`), la fonction `identite(*http.Request) (string, error)` et l'erreur `ErrIdentiteManquante`. L'en-tête est
  découpé de ses espaces ; vide ou absent, il produit `ErrIdentiteManquante`.
- **Ce n'est pas un port du domaine.** Un port pour un seul adaptateur, sans second implémentation réelle, serait
  l'abstraction spéculative que l'ADR 0001 refuse.
- L'adaptateur HTTP passe l'identité **explicitement** au cas d'utilisation (`Consulter(ctx, identifiant)`,
  `Enregistrer(ctx, identifiant, saisie)`). Aucun objet « utilisateur courant » ne circule dans l'application, et le cas
  d'utilisation reste testable avec une simple chaîne.
- Aucune authentification applicative : le service ne vérifie pas la preuve, il transporte une identité de confiance.
  L'identifiant reçu devient la clé primaire de `users`, ce qui garantit qu'un appelant n'atteint que sa propre ligne.

## Conséquences

- **Positif :** `401` est produit à un seul endroit (la table `statutHTTP`), et chaque route se contente d'appeler
  `identite` ; remplacer le transport (jeton signé, en-tête différent, passerelle amont) ne touche qu'un fichier ; le cas
  d'utilisation ne connaît ni `http.Request` ni en-tête.
- **Négatif :** chaque nouvelle route doit penser à appeler `identite` — le compilateur ne l'impose pas.
- **Limite assumée :** n'importe quel client connaissant un identifiant peut agir en son nom. C'est une hypothèse du
  sujet, pas un oubli ; elle est reprise dans les limites connues de [architecture.md](../architecture.md).

## Alternatives écartées

- **Port `IdentityProvider` déclaré par le domaine** : c'était la forme retenue avant le portage ; avec un seul adaptateur
  et aucune vérification, l'interface n'apportait qu'une indirection.
- **Lire `X-User` directement dans chaque gestionnaire** : duplique la règle du `401` et laisse passer un en-tête vide
  comme un identifiant valide.
- **Authentification interne** (mot de passe, session) : réintroduit un cas d'utilisation explicitement retiré du
  périmètre.
