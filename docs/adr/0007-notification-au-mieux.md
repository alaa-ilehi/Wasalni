# ADR 0007 - Notification au mieux, sans remise en cause de la réservation

- **Statut :** accepté
- **Date :** 2026-11-24

## Contexte

Le système de notification est un acteur secondaire externe (UC12). Il peut être indisponible, lent, ou refuser un
message. Or une réservation est déjà validée en base au moment où la notification part : si l'échec remontait, l'appelant
verrait une erreur alors que la place est réellement retenue, et rejouer la requête donnerait un doublon.

## Décision

- Le port `NotificationPort.send(destinataire, message, type_evenement)` est de type **au mieux** : il ne propage
  aucune exception.
- L'appel est encadré dans `BookSeat._notify` : `try/except Exception` + `logger.warning(...)`. Une notification non
  livrée est donc visible dans les journaux, sans effet sur la transaction déjà validée.
- Aucune file d'attente, aucune nouvelle tentative automatique, aucun stockage du message : le contenu n'est pas conservé.
- L'ordre est toujours « commit, puis notification » ; jamais l'inverse.

## Conséquences

- **Positif :** un incident du système externe ne dégrade ni la réservation ni le code HTTP ; le comportement est
  vérifiable en test (`test_notification_failure_keeps_the_booking` : un port qui lève laisse la réservation `ACTIVE`
  et la place décrémentée).
- **Négatif :** un message peut être perdu sans trace au-delà du journal applicatif ; aucune garantie de livraison.
- **Point d'extension :** si UC12 devait devenir fiable, la décision à prendre est une file persistante (outbox),
  pas un `try/except` plus large.

## Alternatives écartées

- **Propager l'échec (transaction distribuée)** : la réservation serait validée côté base mais renvoyée en erreur,
  et un nouvel essai créerait un doublon.
- **Nouvelle tentative immédiate dans le cas d'utilisation** : allonge la requête HTTP et ne corrige pas une panne réelle.
- **Écrire un enregistrement « à notifier » en base** : introduit une table d'outbox pour un acteur simulé.
