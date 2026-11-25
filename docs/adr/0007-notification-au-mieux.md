# ADR 0007 - Notification au mieux, sans remise en cause de l'écriture

- **Statut :** appliqué pour UC3, UC5 et UC11 — l'adaptateur se limite à un journal ; le système externe d'UC12 n'existe pas
- **Date :** 2026-11-24

## Contexte

Le système de notification est un acteur secondaire externe (UC12). Il peut être indisponible, lent, ou refuser un
message. Or l'écriture est déjà validée en base au moment où la notification part : si l'échec remontait, l'appelant
verrait une erreur alors que l'écriture est réellement faite, et rejouer la requête créerait un doublon.

UC3 (réserver une place), UC5 (annuler une réservation) et UC11 (terminer un trajet) émettent une notification après
l'écriture ; UC8 (gérer son profil) n'en prévoit pas. UC5 journalise l'événement pour le passager et le conducteur, UC11
pour les passagers, sans livraison externe.

## Décision

Conservée telle quelle, pour la future implémentation de UC12 :

- Le port de notification est de type **au mieux** : il ne propage aucune erreur.
- L'appel est encadré chez l'appelant (journal d'avertissement) : une notification non livrée reste visible dans les
  journaux, sans effet sur la transaction déjà validée.
- Aucune file d'attente, aucune nouvelle tentative automatique, aucun stockage du message : le contenu n'est pas conservé.
- L'ordre est toujours « validation de l'écriture, puis notification » ; jamais l'inverse.
- Quand il sera écrit, le port sera déclaré par `internal/application` — comme `NormaliseurPhoto` l'est aujourd'hui — et
  non par le domaine, qui n'a aucune raison de connaître un canal de diffusion.

Ces règles ont un code depuis UC3 : les ports sont déclarés par `internal/application` (`Notificateur` pour UC3,
`NotificateurAnnulation` pour UC5, `NotificateurFinTrajet` pour UC11) et l'adaptateur
`internal/infrastructure/notifications/journal.go` se contente d'écrire une ligne de journal.

## Conséquences

- **Aujourd'hui :** la réservation, son annulation et la terminaison du trajet sont validées avant l'événement de
  journal ; un échec du port ne défait pas l'écriture et ne change pas la réponse HTTP (tests UC3, UC5 et UC11).
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
