# ADR 0011 - Authentification par session, mots de passe hachés en standard

- **Statut :** appliqué (UC13)
- **Date :** 2026-11-25

## Contexte

L'ADR 0002 a délégué UC13 à un système externe : le service lisait l'en-tête `X-User` et le croyait sur parole.
Tant que l'API ne modifiait que des données fictives, la limite était supportable. Elle ne l'est plus : `X-User` permet
d'annuler la réservation d'autrui, de supprimer le trajet d'un conducteur ou de déposer une évaluation en son nom. Un
client n'a besoin que de connaître un identifiant public — un conducteur est visible dans `GET /rides/{id}`.

Il faut donc une preuve d'identité. Trois familles étaient possibles : une session serveur, un jeton signé
auto-porteur, une clé d'API partagée. Le service n'a pas de fournisseur d'identité disponible et ne doit pas ajouter de
dépendance (ADR 0009) : la solution doit tenir dans la bibliothèque standard.

## Décision

### Sessions serveur, jeton opaque

- Trois routes publiques de la frontière HTTP : `POST /auth/register` (`201`), `POST /auth/login` (`200`) et
  `DELETE /auth/session` (`204`).
- Le jeton est **32 octets aléatoires** en base64url, rendu **une seule fois** au client. La base ne conserve que son
  empreinte SHA-256 (`sessions.token_hash`) : une copie de la base ne permet donc pas de rejouer une session.
- Durée de vie **24 h**, stockée dans `sessions.expires_at`. La purge est **paresseuse** : `ParJeton` supprime les
  lignes échues avant de lire, ce qui évite une tâche périodique pour un volume négligeable.
- La déconnexion supprime la ligne : le jeton devient inutilisable immédiatement, sans attendre l'expiration.

### Mots de passe : PBKDF2-HMAC-SHA256, sans dépendance

- **Sel aléatoire de 16 octets par compte**, tiré à l'inscription. Deux comptes qui choisissent le même mot de passe
  obtiennent donc deux condensés différents : une base volée ne permet pas de repérer les mots de passe partagés, et un
  attaquant ne peut pas précalculer une table pour l'ensemble des comptes.
- **120 000 itérations**, au-dessus du plancher de 100 000 retenu, en deçà du coût qui rendrait chaque connexion
  pénible (~100 ms mesurés). Sel, condensé et **nombre d'itérations sont stockés par ligne** : le coût pourra augmenter
  plus tard sans invalider les comptes existants.
- La vérification utilise `crypto/subtle.ConstantTimeCompare` : la durée ne dépend pas du nombre d'octets corrects.
- PBKDF2 (RFC 8018) est écrit sur `crypto/hmac` et `crypto/sha256`, soit ~30 lignes, et **vérifié contre les vecteurs
  publiés** — RFC 6070 pour la construction (PRF SHA-1) et vecteurs SHA-256 pour le PRF de production. Aucune dépendance
  ajoutée.

### Compte et profil sont deux objets

`credentials` et `sessions` sont **séparées de `users`**, sans clé étrangère vers elle. Deux raisons : un identifiant doit
pouvoir s'inscrire avant d'avoir rempli son profil (UC8 reste facultatif), et un profil créé avant UC13 n'a pas de mot de
passe. L'identité devient donc la clé `credentials.user_id`, le profil une donnée attachée quand elle existe — ce que
`GET /rides/{id}` sait déjà gérer (conducteur sans profil).

### Une seule source d'identité

`internal/api/identite.go` ne lit plus que `Authorization: Bearer <jeton>` et le résout par `Authentification.Resoudre`.
L'en-tête `X-User` n'est plus accepté nulle part. **Toutes les routes sauf `/health` et les trois routes `/auth/*`
exigent une session**, y compris la recherche et le détail d'un trajet : la règle est uniforme, donc vérifiable route par
route dans un seul test.

Le mapping des erreurs reste dans la table `statutHTTP` (ADR 0006) : `ErrIdentifiantsInvalides` et `ErrSessionInvalide`
→ `401`, `ErrIdentifiantDejaInscrit` → `409`, `ErrMotDePasseInvalide` → `400`. Un identifiant inconnu et un mot de passe
faux rendent **le même message** : la réponse ne dit pas lequel des deux existe.

## Conséquences

- **Positif :** l'identité est prouvée ; un jeton volé expire en 24 h et la déconnexion le tue ; la base seule ne
  contient ni jeton utilisable ni mot de passe en clair ; aucune dépendance, aucun secret de configuration à gérer ; le
  coût de PBKDF2 est le même sur toutes les plateformes, sans cgo.
- **Positif :** la séparation compte/profil évite une migration des `users` existants — s'inscrire suffit.
- **Négatif :** chaque connexion paie ~100 ms de calcul, et chaque requête authentifiée une lecture SQLite (une ligne,
  indexée par la clé primaire).
- **Négatif :** pas de rafraîchissement de jeton ; au bout de 24 h le client doit se reconnecter. Pas de révocation
  globale (changer un mot de passe ne tue pas les sessions ouvertes), pas de limitation du nombre de tentatives : un
  attaquant peut essayer en ligne à ~10 essais par seconde et par cœur. Acceptable pour un service de démonstration,
  à traiter avant une exposition publique.
- **Négatif :** le corps des requêtes doit désormais porter un jeton ; tous les exemples `curl` et le futur client web
  doivent se connecter d'abord.

## Alternatives écartées

- **JWT HS256 auto-porteur** : pas de lecture SQLite par requête, mais aucune révocation immédiate (la déconnexion ne
  peut qu'attendre l'expiration), un secret de signature à distribuer et à faire tourner, et une bibliothèque ou du code
  de validation de claims à écrire. Les sessions serveur coûtent une ligne par utilisateur connecté.
- **Clé d'API partagée + `X-User`** : une seule clé pour tous les clients, donc n'importe quel porteur peut se déclarer
  n'importe qui — cela ne corrige rien.
- **Cookie de session** : plus simple pour un navigateur, inadapté à un client non-navigateur et source de CSRF ; le
  service est une API.
- **`golang.org/x/crypto/bcrypt` ou `argon2id`** : meilleurs algorithmes sur le papier, mais une nouvelle dépendance
  directe pour un besoin que PBKDF2 couvre, alors que l'ADR 0009 impose de justifier chaque dépendance. `argon2id`
  restera la bonne réponse si le service devient public.
- **Jeton stocké en clair** : rendrait toute copie de la base suffisante pour usurper les sessions ouvertes.
- **Sel unique global** : permettrait de comparer les comptes entre eux ; le sel par compte est la seule version utile.
