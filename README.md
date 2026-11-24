# Wasalni — TP1 Processus unifié et approches agiles

**Énoncé :** cadrage du système « Covoiturage » (voir `enonce.pdf`).
**Système choisi :** plateforme de covoiturage interurbain (API web).
**Nombre de UC :** 12, dont un à risque technique (réservation concurrente : UC3).
**Stack :** Go + SQLite (bibliothèque standard et `modernc.org/sqlite`, sans cgo).
**État actuel :** UC8 (gérer son profil) et UC1 (publier un trajet) sont complets ; UC3 (réserver une place) est
implémenté sans décrément de places — la table `rides` existe, mais son compteur ne bouge pas encore, donc le décrément
atomique et son test de concurrence restent à faire, comme le note [docs/architecture.md](docs/architecture.md) §15. Les
tests se lancent avec `go test ./...`.

Documents de référence : [docs/vision.md](docs/vision.md) (vision, acteurs, UC, priorisation, plan),
[docs/use-case-elaboration.md](docs/use-case-elaboration.md) (fiches détaillées, diagramme de cas d'utilisation) et
[docs/architecture.md](docs/architecture.md) (architecture du service Go, ADR, modèle de données).

---

## 1. Vision (5 lignes) et acteurs

- **Problème :** les conducteurs roulent avec des places vides alors que les passagers peinent à trouver un transport interurbain fiable et abordable.
- **Utilisateurs :** conducteurs et passagers — deux rôles qu'un utilisateur identifié peut occuper sur différents trajets.
- **Valeur :** publier ou trouver un trajet en quelques clics et retenir une place.
- **Périmètre :** API web simple, une seule plateforme, trajets interurbains uniquement, identité fournie par un système d'authentification externe (en-tête `X-User`). L'interface utilisateur est entièrement en français.
- **Succès :** aucun trajet surréservé ; chaque réservation est soit active, soit libérée avant l'échéance définie par le conducteur.

### Acteurs

| Acteur | Type | Rôle |
|---|---|---|
| Utilisateur | abstrait (généralisation) | rôle commun : profil, évaluations et historique des trajets |
| Conducteur | principal (`Utilisateur`) | publie, modifie, supprime et termine des trajets |
| Passager | principal (`Utilisateur`) | recherche, réserve, annule une réservation, évalue |
| Système d'authentification | secondaire (externe) | fournit l'identité transportée dans `X-User` |
| Système de notification | secondaire (externe) | informe les deux parties des changements de statut |

---

## 2. Liste des UC

| ID | Cas d'utilisation | Remarque |
|---|---|---|
| UC1 | Publier un trajet | inclut le délai d'annulation X |
| UC2 | Rechercher des trajets | |
| UC3 | Réserver une place | **risque technique 1** : course sur la dernière place |
| UC5 | Annuler une réservation | libère la place ; échéance = départ - X |
| UC6 | S'évaluer mutuellement | conducteur et passager s'évaluent après la fin du trajet |
| UC7 | Consulter les détails d'un trajet | trajet + profil du conducteur + évaluation avant réservation |
| UC8 | Gérer son profil | créer et modifier nom, âge, sexe, photo |
| UC9 | Modifier ou supprimer un trajet publié | propriétaire uniquement ; la suppression annule les réservations |
| UC10 | Voir mes trajets / réservations | point d'entrée pour annuler et évaluer |
| UC11 | Terminer un trajet | le conducteur marque le trajet terminé ; débloque UC6 |
| UC12 | Recevoir des notifications | système de notification externe ; livraison au mieux |
| UC13 | S'authentifier | système d'authentification externe ; source de `X-User` |

La réservation (UC3) et l'annulation (UC5) sont deux UC distincts : UC3 retient la place, UC5 la libère.

---

## 3. Priorisation

Trois critères (énoncé) : valeur métier, risque, impact sur l'architecture. Échelle : 1 = faible, 3 = fort.
Niveau : **haute** (7-9), **moyenne** (5-6), **faible** (3-4).

| UC | Valeur métier | Risque | Impact architecture | Total | Niveau | Ordre |
|---|---|---|---|---|---|---|
| UC3 Réserver une place | 3 | 3 | 3 | **9** | haute | 1 |
| UC1 Publier un trajet | 3 | 1 | 3 | **7** | haute | 2 |
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

### UC à traiter en premier

- **UC3** : il porte le risque de concurrence et impose la structure du cœur du système (trajets, places, réservations).
- **UC1 et UC2** viennent ensuite : ils sont simples, mais nécessaires pour disposer de trajets à réserver.
- L'authentification (UC13) et les notifications (UC12) sont déléguées à des systèmes externes.

---

## 4. Planification : 3 itérations et 4 phases

La phase **Inception** (points 1 à 4) a lieu avant l'itération 1.

| Itération | Contenu | Phase | Jalon (UP) | Milestone GitHub | Tag Git |
|---|---|---|---|---|---|
| 0 | Vision, UC, dépôt, diagramme de cas d'utilisation | Inception | LCO | Inception | `v0.0-inception` |
| 1 | Architecture de base, UC3 de bout en bout, premier test en CI | Élaboration | LCA | Elaboration | `v0.1-elaboration` |
| 2 | UC13, UC1, UC2, UC7, UC8 | Construction | IOC | Construction | `v0.2-construction` |
| 3 | UC5, UC9, UC10, UC11, UC12, UC6, corrections, revue croisée, livraison | Transition | PR | Transition | `v1.0-transition` |

### Critères de sortie des jalons

| Jalon | Critère |
|---|---|
| LCO | vision et UC validés, dépôt et tickets créés |
| LCA | une route de bout en bout (contrôleur, service, accès aux données) tourne pour UC3, avec un test vert en CI |
| IOC | les UC de l'itération 2 fonctionnent, avec leurs tests |
| PR | tous les UC prévus sont livrés, la revue croisée est faite, la CI est verte |

### Correspondance milestones GitHub, branches et tags

| Milestone | UC (tickets) | Branches | Tag |
|---|---|---|---|
| Inception | cadrage | `main` | `v0.0-inception` |
| Elaboration | UC3 (route de base) + architecture | `architecture-elaboration` | `v0.1-elaboration` |
| Construction | UC13, UC1, UC2, UC7, UC8 | `uc13-authenticate`, `uc1-post-ride`, `uc2-search-rides`, `uc7-ride-details`, `uc8-manage-profile` | `v0.2-construction` |
| Transition | UC5, UC9, UC10, UC11, UC12, UC6 | `uc5-cancel-booking`, `uc9-edit-ride`, `uc10-my-rides`, `uc11-complete-ride`, `uc12-notifications`, `uc6-rate-each-other` | `v1.0-transition` |

---

## 5. Automatisation

`scripts/elaboration.sh` rejoue les étapes 1 à 4 de la partie 1 avec `git` et `gh`
(tickets, milestones, tableau de bord, branche `architecture-elaboration`, CI, tag `v0.1-elaboration`).
`docs/elaboration-manual.md` donne les mêmes étapes à la main.

Le contrôle local est `go vet ./...` puis `go test ./...` ; ce sont les deux commandes que la CI exécute.

a7la nes gafsa
