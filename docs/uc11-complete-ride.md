# UC11 - Terminer un trajet

- **Acteur principal :** Conducteur. **Acteur secondaire :** Système de notification (UC12).
- **Préconditions :** le trajet existe, appartient au conducteur et est `PUBLISHED`.
- **Postconditions :** le trajet est `COMPLETED`; les réservations ne changent pas.

Séquence : [uml/uc11-sequence.puml](uml/uc11-sequence.puml). Modèle de classes : [uml/class-model.puml](uml/class-model.puml).

## Scénario nominal

1. Le conducteur appelle `POST /rides/{id}/complete` avec son jeton de session.
2. L'API vérifie l'identité et délègue au cas d'utilisation UC11.
3. Le dépôt vérifie le propriétaire et l'état, puis effectue la transition `PUBLISHED` vers `COMPLETED` dans une transaction.
4. Après commit, l'événement de fin est journalisé au mieux; l'API répond `200` avec le trajet terminé.

## Scénarios alternatifs

| Condition | Résultat |
|---|---|
| Identité absente | `401`, aucune écriture |
| Trajet inconnu | `404`, aucune écriture |
| Conducteur différent du propriétaire | `403`, aucune écriture |
| Trajet déjà terminé | `409`, aucune nouvelle transition |
| Journal d'événement en échec | trajet terminé conservé, réponse `200` |

## Règles et tests

- La transition est définitive et irréversible; aucun statut de réservation n'est modifié.
- Une comparaison conditionnelle et une transaction empêchent une seconde transition sous appels concurrents.
- L'événement part après commit, au mieux : `POST` JSON vers le notificateur configuré, journal sinon (ADR 0007, ADR 0013).
- Tests domaine/application/SQLite/HTTP couvrent propriété, état, idempotence négative, concurrence, codes HTTP et échec de notification.
