# Wasalni# TP1 – Processus unifié et approches agiles

## Énoncé : cadrage du système « Covoiturage »

**Système choisi :** plateforme de covoiturage interurbain (API web).
**Nombre de UC :** 6, dont deux à risque technique (réservation concurrente et paiement).

---

## 1. Vision (5 lignes) et acteurs

- **Problème :** des conducteurs roulent avec des places vides, alors que les passagers peinent à trouver un transport interurbain fiable et abordable.
- **Utilisateurs :** conducteurs et passagers.
- **Valeur :** publier ou trouver un trajet en quelques clics, bloquer une place, payer en ligne, sans jamais surréserver un véhicule.
- **Périmètre :** API web simple, une plateforme, trajets interurbains uniquement.
- **Succès :** zéro trajet surréservé, et chaque réservation est soit payée et confirmée, soit libérée.

### Acteurs

| Acteur | Type | Rôle |
|---|---|---|
| Conducteur | principal | publie des trajets |
| Passager | principal | recherche, réserve, paie, annule, note |
| Service de paiement | secondaire (simulé) | débite la carte du passager |
| Service de notification | secondaire (simulé) | informe conducteur et passager |

> L'identification de l'utilisateur est simplifiée (en-tête `X-User`) : il n'y a pas de UC d'authentification.

---

## 2. Liste des UC

| ID | Cas d'utilisation | Remarque |
|---|---|---|
| UC1 | Publier un trajet | |
| UC2 | Rechercher des trajets | |
| UC3 | Réserver une place | **UC à risque technique n°1** : course sur la dernière place. La réservation passe en `EN ATTENTE` et la place est bloquée. |
| UC4 | Payer une réservation | **UC à risque technique n°2** : paiement externe et gestion des échecs. `EN ATTENTE` passe à `CONFIRMÉE`. |
| UC5 | Annuler une réservation | libère la place, rembourse si la réservation est payée |
| UC6 | Noter le conducteur | |

La réservation (UC3) et le paiement (UC4) sont deux UC distincts : la place est d'abord bloquée, puis confirmée par le paiement.

---

## 3. Priorisation

Échelle : 1 = faible, 3 = fort.

| UC | Valeur métier | Risque | Impact sur l'architecture | Total | Ordre |
|---|---|---|---|---|---|
| UC3 Réserver une place | 3 | 3 | 3 | **9** | 1 |
| UC4 Payer une réservation | 3 | 3 | 2 | **8** | 2 |
| UC1 Publier un trajet | 3 | 1 | 3 | 7 | 3 |
| UC2 Rechercher des trajets | 3 | 1 | 2 | 6 | 4 |
| UC5 Annuler une réservation | 2 | 2 | 1 | 5 | 5 |
| UC6 Noter le conducteur | 1 | 1 | 1 | 3 | 6 |

### UC à traiter en premier

- **UC3** : il porte le risque de concurrence et impose la structure du cœur du système (trajets, places, réservations).
- **UC4** : il porte le risque d'intégration externe (paiement et gestion des échecs).
- **UC1 et UC2** viennent ensuite : ils sont simples, mais nécessaires pour disposer de trajets à réserver.

---

## 4. Planification : 3 itérations et 4 phases

La phase **Inception** (points 1 à 4) a lieu avant l'itération 1.

| Phase | Itération | Contenu | Jalon (UP) | Tag Git |
|---|---|---|---|---|
| Inception | avant l'itération 1 | vision, acteurs, UC, priorisation, dépôt, diagramme de cas d'utilisation | LCO (objectifs du cycle de vie) | `v0.0-inception` |
| Élaboration | **Itération 1** | architecture de base, UC3 de bout en bout, premier test automatisé en CI | LCA (architecture validée) | `v0.1-elaboration` |
| Construction | **Itération 2** | UC4 (paiement), UC1, UC2 | IOC (capacité opérationnelle initiale) | `v0.2-construction` |
| Construction → Transition | **Itération 3** | UC5, UC6, corrections, revue croisée, livraison | PR (version livrée) | `v1.0-transition` |

### Critères de sortie des jalons

| Jalon | Critère |
|---|---|
| LCO | vision et UC validés, dépôt et tickets créés |
| LCA | une route de bout en bout (contrôleur, service, accès aux données) tourne pour UC3, avec un test vert en CI |
| IOC | le parcours réserver puis payer fonctionne, avec ses tests |
| PR | tous les UC prévus sont livrés, la revue croisée est faite, la CI est verte |

### Correspondance milestones GitHub, branches et tags

| Milestone | UC (ticket) | Branche | Tag |
|---|---|---|---|
| Inception | cadrage | `main` | `v0.0-inception` |
| Elaboration | UC3 | `uc3-book-seat` | `v0.1-elaboration` |
| Construction | UC4, UC1, UC2 | `uc4-pay-booking`, `uc1-post-ride`, `uc2-search-rides` | `v0.2-construction` |
| Transition | UC5, UC6 | `uc5-cancel-booking`, `uc6-rate-driver` | `v1.0-transition` |
