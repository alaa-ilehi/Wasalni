# ADR 0009 - Go comme langage du service

- **Statut :** accepté
- **Date :** 2026-11-24

## Contexte

Le backend a été porté de Python vers Go par décision du porteur du projet ; l'application Python est retirée du
dépôt. Le service doit rester un binaire unique, sans infrastructure serveur, sans chaîne de compilation C en CI, et
livrable en une commande. UC8 (gérer son profil) est le premier cas d'utilisation livré dans ce langage.

## Décision

- **Go**, avec le module déclaré `go 1.26.0`. Les motifs de méthode utilisés pour le routage datent de Go 1.22.
- **`net/http` avec les motifs de méthode** : `mux.HandleFunc("GET /profile", …)`, `mux.HandleFunc("PUT /profile", …)`.
  La méthode et le chemin tiennent dans le motif, sans routeur tiers.
- **`database/sql`** comme unique accès aux données, avec **`modernc.org/sqlite`** (pur Go, sans cgo) — ADR 0005.
- **Bibliothèque standard pour le reste** : `encoding/json` avec `DisallowUnknownFields`, `image` et ses décodeurs
  d'entrée `image/png`, `image/jpeg`, `image/gif`, `log/slog` pour les journaux, `signal.NotifyContext` pour l'arrêt
  propre, `httptest` pour les tests d'intégration.
- **Une seconde dépendance directe, décidée explicitement : `github.com/SeriousBug/webp-go-pure`** (ADR 0008). La
  bibliothèque standard ne sait pas encoder le WebP, et le gain de poids est mesuré : la photo de profil passe de
  787 545 à 179 256 octets sur la fixture de test. Le module est pur Go, sans cgo, et fournit encodeur et décodeur. La
  règle « bibliothèque standard seule » reste la référence ; c'est la seule dérogation, et elle est consignée ici.
- La racine de composition est un seul fichier, `cmd/wasalni/main.go` : ouverture de la base, câblage du cas
  d'utilisation et du normaliseur de photo, serveur HTTP, arrêt propre.

## Conséquences

- **Positif :** un binaire unique, sans interpréteur, sans environnement virtuel et sans dépendance native ; `go vet ./...`
  et `go test ./...` suffisent en local comme en CI ; les motifs de méthode couvrent les trois routes sans routeur
  externe ; les erreurs sont des valeurs comparables par `errors.Is` (ADR 0006) ; le compilateur impose les frontières de
  paquets (ADR 0001).
- **Négatif :** l'écosystème de la bibliothèque standard est pauvre en codecs d'image — pas d'encodeur WebP ni AVIF, d'où
  la dépendance ajoutée pour le WebP (ADR 0008) ; pas d'outil de migration, le schéma reste idempotent et appliqué au
  démarrage ; les requêtes SQL sont écrites et converties à la main.
- **Effet sur l'outillage :** le contrôle local et la CI exécutent `go vet ./...` puis `go test ./...` (voir
  [elaboration-manual.md](../elaboration-manual.md)).

## Alternatives écartées

- **Python (statu quo)** : écarté par décision du porteur du projet, en même temps que la suppression du backend
  existant.
- **Un framework web tiers (Gin, Echo, Chi)** : apporte du routage et des intergiciels dont trois routes n'ont pas
  besoin ; `net/http` sait déjà router par méthode depuis Go 1.22.
- **Un ORM** : masquerait le SQL qui porte la décision de concurrence (ADR 0004) pour une seule table (ADR 0005).
- **Rust ou Node** : mêmes propriétés de service, mais coût d'apprentissage et outillage de test sans gain sur les
  critères du projet.
