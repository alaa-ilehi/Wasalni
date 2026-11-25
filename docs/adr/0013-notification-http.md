# ADR 0013 - Livraison des notifications par webhook HTTP

- **Statut :** appliqué — l'adaptateur `notifications.Webhook` livre chaque événement par `POST` JSON, le journal reste
  le repli quand aucune URL n'est configurée
- **Date :** 2026-11-25

## Contexte

ADR 0007 a fixé la sémantique : la notification est « au mieux », elle intervient après le commit et son échec ne
remet pas en cause l'opération métier. Il manquait l'envoi. Le journal (`notifications.Journal`) écrivait une ligne
`slog` par événement, ce qui ne prévient personne.

UC12 est le système externe de notification, hors périmètre du service. Le service doit donc livrer l'événement à un
destinataire que l'exploitant choisit, sans imposer de fournisseur, et sans ajouter de dépendance : `go.mod` ne compte
que `webp-go-pure` et `modernc.org/sqlite` (ADR 0005, ADR 0009).

## Décision

Un adaptateur HTTP, écrit avec la bibliothèque standard (`net/http`, `encoding/json`), livre chaque événement par un
`POST` **JSON** vers une URL unique.

| Méthode appelée | `event` | Champs |
|---|---|---|
| `NotifierReservation` | `reservation_accepted` | `reservation`, `ride`, `passenger` |
| `NotifierAnnulation` | `reservation_cancelled` | `reservation`, `ride`, `passenger` |
| `NotifierFinTrajet` | `ride_completed` | `ride`, `driver` |
| `NotifierModificationTrajet` | `ride_modified` | `ride`, `driver` |
| `NotifierSuppressionTrajet` | `ride_deleted` | `ride`, `driver` |

```json
{"event":"reservation_accepted","reservation":"r1","ride":"t1","passenger":"alice"}
```

Contrat de livraison :

- `Content-Type: application/json`, corps encodé une fois, jamais réessayé ;
- client borné par `DelaiLivraison` (5 s) : une cible lente ne retient pas la requête métier plus longtemps ;
- `2xx` rend `nil` ; connexion refusée, délai dépassé, réponse non-`2xx` ou URL invalide rendent une erreur, que le cas
  d'usage journalise déjà sans annuler l'opération (ADR 0007) ;
- le corps de la réponse est toujours lu et jeté, pour que la connexion reste réutilisable.

Le choix de l'adaptateur se fait **une seule fois au démarrage** (`cmd/wasalni/main.go`) : `WASALNI_NOTIFICATION_URL`
définie câble le webhook, absente laisse le journal. Les cas d'usage, les ports et le domaine ne changent pas.

L'URL accepte n'importe quel destinataire capable de recevoir un `POST` JSON : un récepteur maison, `ntfy`, `Gotify`, un
webhook Slack ou Discord via un relais qui traduit le corps.

## Conséquences

- Un déploiement sans URL garde exactement le comportement précédent : les événements restent lisibles dans le journal.
- Une cible lente ajoute au plus 5 s à la requête qui a déclenché l'événement ; c'est le prix d'un envoi synchrone, et il
  reste borné. Une file d'attente persistante serait la réponse si l'envoi devait survivre à un redémarrage.
- Aucun réessai : un événement raté est perdu au-delà de la ligne de journal. C'est cohérent avec « au mieux ».
- Le corps est un contrat public minimal (cinq noms d'événements, des identifiants) : un changement de nom d'événement
  demanderait de prévenir les destinataires.

## Alternatives écartées

- **SMTP (`net/smtp`)** : impose un serveur de mail et un format de message pour un besoin qui se résume à « prévenir
  quelqu'un » ; les tests exigeraient un faux serveur SMTP.
- **Table outbox en base** : garantit la livraison après redémarrage, mais ajoute une table, un ordonnanceur et une
  purge — beaucoup de code pour un TP où la perte d'un événement ne casse rien.
- **Bibliothèque de notification tierce** : une dépendance de plus pour quelques dizaines de lignes, alors que
  l'invariant « deux dépendances directes » est documenté.
