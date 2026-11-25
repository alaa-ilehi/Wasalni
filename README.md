# Wasalni — TP1 Processus unifié et approches agiles

**Énoncé :** cadrage du système « Covoiturage » (voir `enonce.pdf`).
**Système choisi :** plateforme de covoiturage interurbain (API web).
**Nombre de UC :** 12, dont un à risque technique (réservation concurrente : UC3).
**Stack :** Go + SQLite (bibliothèque standard et `modernc.org/sqlite`, sans cgo).
**État actuel :** UC8 (profil), UC1 (publier), UC2 (rechercher), UC7 (détails), UC3 (réserver), UC5 (annuler), UC6
(évaluer), UC9 (modifier/supprimer), UC10 (mes trajets et réservations), UC11 (terminer un trajet) et UC13 (s'authentifier)
sont livrés ; UC12 reste partiel (journal au lieu d'un système de notification).
UC3 consomme une place par un décrément conditionnel dans la même transaction que l'écriture, sans surréservation possible
(ADR 0004) ; `bookings.ride_id` référence `rides(id)`. UC6 enregistre une note par couple sur un trajet terminé et alimente
`driver.rating` dans UC7 (ADR 0010) ; UC11 fournit maintenant la transition vers `COMPLETED`. UC13 remplace l'en-tête de
confiance qui existait avant par une session : mots de passe hachés en PBKDF2-HMAC-SHA256 avec un sel aléatoire par compte, jeton
opaque rendu une seule fois et conservé haché, `Authorization: Bearer` exigé sur toutes les routes sauf `/health` et
`/auth/*` (ADR 0011). La photo de profil est
réencodée en **WebP sans perte** de 512 px et 512 Kio au plus, au-delà d'un budget de
pixels qui refuse les bombes de décompression (ADR 0008).

Les tests se lancent avec `go test -race ./...` (la CI fait de même, après `go vet` et un contrôle `gofmt`).

```bash
go run ./cmd/wasalni                                   # écoute sur :8080, base wasalni.db

# 1. s'inscrire : le jeton n'est rendu qu'ici, une seule fois
curl -s -X POST -d '{"id":"amine","password":"motdepasse"}' localhost:8080/auth/register
#    -> 201 {"id":"amine","token":"<jeton>"}  (refusé en 409 si l'identifiant est déjà pris)

# 2. se connecter : même réponse, un nouveau jeton (401 si identifiant ou mot de passe faux)
JETON=$(curl -s -X POST -d '{"id":"amine","password":"motdepasse"}' \
        localhost:8080/auth/login | sed -E 's/.*"token":"([^"]+)".*/\1/')

# 3. appeler une route protégée (toutes le sont, sauf /health et /auth/*)
curl -s -H "Authorization: Bearer $JETON" localhost:8080/profile
curl -s -H "Authorization: Bearer $JETON" localhost:8080/rides            # page de 50 trajets au plus
curl -s -H "Authorization: Bearer $JETON" 'localhost:8080/rides?limit=2'  # puis la page suivante :
curl -s -H "Authorization: Bearer $JETON" 'localhost:8080/rides?limit=2&after=<next_cursor>'
curl -s -H "Authorization: Bearer $JETON" localhost:8080/me/rides      # mes trajets (UC10)
curl -s -H "Authorization: Bearer $JETON" localhost:8080/me/bookings   # mes réservations (UC10)

# 4. fermer la session : le jeton devient inutilisable
curl -s -o /dev/null -w '%{http_code}\n' -X DELETE -H "Authorization: Bearer $JETON" localhost:8080/auth/session
```

Documents de référence : [docs/vision.md](docs/vision.md) (vision, acteurs, UC, priorisation, plan),
[docs/use-case-elaboration.md](docs/use-case-elaboration.md) (fiches détaillées, diagramme de cas d'utilisation) et
[docs/architecture.md](docs/architecture.md) (architecture du service Go, ADR, modèle de données),
[docs/uc3-book-seat.md](docs/uc3-book-seat.md), [docs/uc6-rate-participants.md](docs/uc6-rate-participants.md) et
[docs/uc11-complete-ride.md](docs/uc11-complete-ride.md).

---

## 1. Vision (5 lignes) et acteurs

- **Problème :** les conducteurs roulent avec des places vides alors que les passagers peinent à trouver un transport interurbain fiable et abordable.
- **Utilisateurs :** conducteurs et passagers — deux rôles qu'un utilisateur identifié peut occuper sur différents trajets.
- **Valeur :** publier ou trouver un trajet en quelques clics et retenir une place.
- **Périmètre :** API web simple, une seule plateforme, trajets interurbains uniquement, identité prouvée par une session délivrée par le service (`Authorization: Bearer`), après inscription ou connexion. L'interface utilisateur est entièrement en français.
- **Succès :** aucun trajet surréservé ; chaque réservation est soit active, soit libérée avant l'échéance définie par le conducteur.

### Acteurs

| Acteur | Type | Rôle |
|---|---|---|
| Utilisateur | abstrait (généralisation) | rôle commun : profil, évaluations et historique des trajets |
| Conducteur | principal (`Utilisateur`) | publie, modifie, supprime et termine des trajets |
| Passager | principal (`Utilisateur`) | recherche, réserve, annule une réservation, évalue |
| Système d'authentification | secondaire (intégré depuis UC13) | délivre les sessions ; le service porte lui-même les comptes (ADR 0011) |
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
| UC13 | S'authentifier | inscription, connexion, déconnexion ; jeton `Authorization: Bearer` (ADR 0011) |

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
- Les notifications (UC12) sont déléguées à un système externe ; l'authentification (UC13) est portée par le service depuis UC13 (ADR 0011).

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

Le contrôle local est `go vet ./...` puis `go test -race ./...`, plus un contrôle `gofmt` ; ce sont les commandes que la CI exécute (build → vet → format → tests).

a7la nes gafsa
