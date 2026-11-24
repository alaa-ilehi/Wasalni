# ADR 0007 - Notification au mieux, sans remise en cause de l'écriture

- **Statut :** appliqué pour UC3 et UC5 — l'adaptateur se limite à un journal ; le système externe d'UC12 n'existe pas
- **Date :** 2026-11-24

## Contexte

Le système de notification est un acteur secondaire externe (UC12). Il peut être indisponible, lent, ou refuser un
message. Or l'écriture est déjà validée en base au moment où la notification part : si l'échec remontait, l'appelant
verrait une erreur alors que l'écriture est réellement faite, et rejouer la requête créerait un doublon.

UC3 (réserver une place) et UC5 (annuler une réservation) émettent une notification après l'écriture ; UC8 (gérer son
profil) n'en prévoit pas. UC5 journalise l'événement pour le passager et le conducteur, sans livraison externe.

## Décision

Conservée telle quelle, pour la future implémentation de UC12 :

- Le port de notification est de type **au mieux** : il ne propage aucune erreur.
- L'appel est encadré chez l'appelant (journal d'avertissement) : une notification non livrée reste visible dans les
  journaux, sans effet sur la transaction déjà validée.
- Aucune file d'attente, aucune nouvelle tentative automatique, aucun stockage du message : le contenu n'est pas conservé.
- L'ordre est toujours « validation de l'écriture, puis notification » ; jamais l'inverse.
- Quand il sera écrit, le port sera déclaré par `internal/application` — comme `NormaliseurPhoto` l'est aujourd'hui — et
  non par le domaine, qui n'a aucune raison de connaître un canal de diffusion.

Ces règles ont un code depuis UC3 : le port est déclaré par `internal/application` (`Notificateur`) pour UC3 et par
`internal/application` (`NotificateurAnnulation`) pour UC5. L'adaptateur `internal/infrastructure/notifications/journal.go`
se contente d'écrire une ligne de journal.

## Conséquences

- **Aujourd'hui :** une réservation ou son annulation est validée avant l'événement de journal ; un échec du port laisse
  l'écriture intacte et la réponse HTTP inchangée (tests UC3 et UC5).
- **À la réimplémentation :** un incident du système externe ne dégradera ni l'écriture ni le code HTTP ; le comportement
  sera vérifiable en test (un port qui échoue laisse l'écriture intacte et le code de réponse inchangé).
- **Négatif :** un message peut être perdu sans trace au-delà du journal applicatif ; aucune garantie de livraison.
- **Point d'extension :** si UC12 devait devenir fiable, la décision à prendre est une file persistante (outbox), pas un
  encadrement plus large des erreurs.

## Alternatives écartées

- **Propager l'échec (transaction distribuée)** : l'écriture serait validée côté base mais renvoyée en erreur, et un
  nouvel essai créerait un doublon.
- **Nouvelle tentative immédiate dans le cas d'utilisation** : allonge la requête HTTP et ne corrige pas une panne réelle.
- **Écrire un enregistrement « à notifier » en base** : introduit une table d'outbox pour un acteur simulé.
- **Supprimer l'ADR avec le code retiré** : la règle sera nécessaire dès qu'une écriture déclenchera un message.
