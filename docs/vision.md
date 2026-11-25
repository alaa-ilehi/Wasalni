# Vision - Covoiturage (Wasalni)

**Problème :** les conducteurs roulent avec des places vides alors que les passagers peinent à trouver un transport interurbain fiable et abordable.
**Utilisateurs :** conducteurs et passagers — deux rôles qu'un utilisateur identifié peut occuper sur différents trajets.
**Valeur :** publier ou trouver un trajet en quelques clics et retenir une place.
**Périmètre :** API web simple, une seule plateforme, trajets interurbains uniquement, identité prouvée par une session délivrée par le service (`Authorization: Bearer`), après inscription ou connexion. L'interface utilisateur est entièrement en français.
**Succès :** aucun trajet surréservé ; chaque réservation est soit active, soit libérée avant l'échéance définie par le conducteur.

## Acteurs
| Acteur | Type | Rôle |
|---|---|---|
| Utilisateur | abstrait (généralisation) | rôle commun : profil, évaluations et historique des trajets |
| Conducteur | principal (`Utilisateur`) | publie, modifie, supprime et termine des trajets |
| Passager | principal (`Utilisateur`) | recherche, réserve, annule une réservation, évalue |
| Système d'authentification | secondaire (intégré depuis UC13) | délivre les sessions ; le service porte les comptes (ADR 0011) |
| Système de notification | secondaire (externe) | informe les deux parties des changements de statut |

## Cas d'utilisation
| ID | Nom | Remarque |
|---|---|---|
| UC1 | Publier un trajet | inclut le délai d'annulation X |
| UC2 | Rechercher des trajets | |
| UC3 | Réserver une place | **risque technique 1** : course sur la dernière place |
| UC5 | Annuler une réservation | libère la place ; échéance = départ - X |
| UC6 | S'évaluer mutuellement | le conducteur note les passagers, le passager note le conducteur, après la fin du trajet |
| UC7 | Consulter les détails d'un trajet | trajet + profil du conducteur + évaluation avant réservation |
| UC8 | Gérer son profil | créer et modifier nom, âge, sexe, photo |
| UC9 | Modifier ou supprimer un trajet publié | propriétaire uniquement ; la suppression annule les réservations |
| UC10 | Voir mes trajets / réservations | point d'entrée pour annuler et évaluer |
| UC11 | Terminer un trajet | le conducteur marque le trajet comme terminé ; débloque UC6 |
| UC12 | Recevoir des notifications | système de notification externe ; livraison au mieux |
| UC13 | S'authentifier | inscription, connexion, déconnexion ; jeton `Authorization: Bearer` (ADR 0011) |

La réservation et l'annulation sont deux cas d'utilisation distincts : UC3 retient la place, UC5 la libère.

## Priorisation (1 = faible, 3 = fort)
Trois critères (énoncé) : valeur métier, risque, impact sur l'architecture. Le total donne le niveau : **haute** (7-9), **moyenne** (5-6), **faible** (3-4).

| UC | Valeur métier | Risque | Impact architecture | Total | Niveau | Ordre |
|---|---|---|---|---|---|---|
| UC3 Réserver une place | 3 | 3 | 3 | 9 | haute | 1 |
| UC1 Publier un trajet | 3 | 1 | 3 | 7 | haute | 2 |
| UC2 Rechercher des trajets | 3 | 1 | 2 | 6 | moyenne | 3 |
| UC13 S'authentifier | 2 | 2 | 2 | 6 | moyenne | 4 |
| UC8 Gérer son profil | 2 | 1 | 2 | 5 | moyenne | 5 |
| UC5 Annuler une réservation | 2 | 2 | 1 | 5 | moyenne | 6 |
| UC9 Modifier ou supprimer un trajet | 2 | 2 | 1 | 5 | moyenne | 7 |
| UC7 Consulter les détails d'un trajet | 2 | 1 | 1 | 4 | faible | 8 |
| UC10 Voir mes trajets / réservations | 2 | 1 | 1 | 4 | faible | 9 |
| UC11 Terminer un trajet | 1 | 2 | 1 | 4 | faible | 10 |
| UC12 Recevoir des notifications | 2 | 1 | 1 | 4 | faible | 11 |
| UC6 S'évaluer mutuellement | 1 | 1 | 1 | 3 | faible | 12 |

**Décision :** UC3 en premier (risque de concurrence, il structure le cœur du système), puis UC1/UC2 (il faut des trajets à réserver). Les notifications restent déléguées à un système externe ; l'authentification est livrée par le service lui-même (UC13, ADR 0011).

## Plan : 3 itérations, 4 phases
| Itération | Contenu | Phase | Jalon | Milestone GitHub | Tag Git |
|---|---|---|---|---|---|
| 0 | Vision, UC, dépôt, diagramme de cas d'utilisation | Inception | LCO | Inception | `v0.0-inception` |
| 1 | Architecture de base, UC3 de bout en bout, premier test en CI | Élaboration | LCA | Elaboration | `v0.1-elaboration` |
| 2 | UC13, UC1, UC2, UC7, UC8 | Construction | IOC | Construction | `v0.2-construction` |
| 3 | UC5, UC9, UC10, UC11, UC12, UC6, corrections, revue croisée, livraison | Transition | PR | Transition | `v1.0-transition` |
