# ADR 0008 - Photo de profil en WebP sans perte

- **Statut :** accepté
- **Date :** 2026-11-24 (révision : la sortie est passée du PNG au WebP, seconde dépendance assumée)

## Contexte

UC8 accepte une photo de profil. Le contrat HTTP la reçoit en base64 dans le corps de la requête, et la base la stocke en
texte : sans borne, une photo peut faire plusieurs mégaoctets et alourdir chaque lecture de profil.

Deux contraintes se croisent :

1. **le format.** La bibliothèque standard sait encoder du PNG et du JPEG, pas du WebP. Un PNG sans perte est sûr mais
   lourd ; un JPEG est léger mais avec perte.
2. **la surface de dépendances.** Le service visait la bibliothèque standard plus `modernc.org/sqlite` (ADR 0009). Une
   photo de profil qui pèse un tiers de moins est un gain mesurable sur chaque `GET /profile`, mais il se paie d'une
   dépendance supplémentaire.

La première version de cet ADR retenait le PNG précisément pour éviter cette dépendance. Le compromis a été réexaminé :
le gain de poids est réel et vérifié par un test, et il existe désormais un encodeur WebP **sans perte** en Go pur, donc
sans cgo ni bibliothèque système — la contrainte qui motivait le refus ne tient plus pour ce format.

## Décision

Le pipeline vit dans `internal/infrastructure/photos` (`Normaliser`) :

- **WebP sans perte (VP8L) plutôt que PNG ou JPEG.** Le format retenu est le WebP sans perte, encodé par
  `github.com/SeriousBug/webp-go-pure` (sous-paquet `std`, qui parle `image.Image`). Sur la fixture de test de 512 × 512,
  le même contenu passe de 787 545 octets en PNG à 179 256 octets en WebP. Le sans perte reste la règle : la photo est
  une donnée du profil, une dégradation irréversible serait difficile à expliquer comme à tester.
- **Une seule dépendance ajoutée, pure Go.** Le module fournit encodeur *et* décodeur : le test d'aller-retour prouve la
  perte nulle sans seconde dépendance, et il n'exige ni cgo ni assemblage. `CGO_ENABLED=0` reste vert.
- **Côté maximal : 512 px.** Le plus grand côté est ramené à 512 px au plus par moyenne de blocs (box filter) ; le
  redimensionneur est écrit à la main, ce qui évite une bibliothèque de rééchantillonnage. La source est convertie une
  seule fois en `*image.RGBA` (sans copie si elle l'est déjà), puis le filtre lit `Pix` directement : 12,7 ms au lieu de
  79,9 ms pour ramener une image de 2000 × 1500 à 512 px.
- **Poids maximal : 512 Kio.** Trois paliers d'encodage au plus — 512, 256, 128 px — et l'image est réencodée à chaque
  essai. L'échelle est bornée par construction : le coût du pire cas ne dépend plus du nombre de paliers intermédiaires.
- **Budget de pixels : 24 Mpx.** `image.DecodeConfig` lit les dimensions **avant** `image.Decode` ; au-delà du budget,
  la photo est refusée avec `ErrImageTropGrande` (`400`). La limite de 10 Mio du corps borne les octets, pas les pixels :
  sans ce plafond, quelques mégaoctets peuvent déclarer 20 000 × 20 000 pixels et faire allouer des gigaoctets avant la
  réduction.
- **Refus seulement si c'est impossible.** L'image n'est refusée (`ErrImageTropLourde`, `400`) qu'à 128 px si le WebP
  dépasse encore 512 Kio. Une image illisible ou d'un format non reconnu donne `ErrImageIllisible` (`400`).
- **Entrée bornée avant décodage.** Le corps de la requête est limité à 10 Mio (`api.TailleMaxCorps`) par
  `http.MaxBytesReader` ; au-delà, `413`. Le préfixe d'une data URL (`data:image/…;base64,`) est accepté, ainsi que les
  retours à la ligne. PNG, JPEG et GIF restent acceptés **en entrée**.
- **Recadrage côté navigateur prévu plus tard.** Le service ne recadre pas et ne détecte pas de visage : il réduit et
  encode. Le cadrage final appartiendra à l'interface web, hors périmètre aujourd'hui.

## Conséquences

- **Positif :** aucune perte de qualité, résultat déterministe pour une même entrée, poids stocké borné et connu, et
  environ quatre fois plus léger que le PNG sur la fixture de test ; le coût du pire cas du pipeline est borné, en pixels
  décodés comme en nombre d'encodages.
- **Négatif — deuxième dépendance.** Le service n'est plus « bibliothèque standard + pilote SQLite » : il dépend d'un
  module tiers, jeune, dont la correction repose sur un portage communautaire. C'est un risque accepté, consigné dans
  l'ADR 0009, et limité à un seul paquet d'infrastructure.
- **Négatif — le client doit décoder du WebP.** Tous les navigateurs actuels le font, mais un client qui attendait du PNG
  doit être adapté ; le contrat JSON, lui, ne change pas (une chaîne base64 dans `photo`).
- **Négatif — CPU.** Le WebP sans perte demande plus de calcul que le PNG à taille comparable : la réduction de moitié du
  poids se paie en millisecondes d'encodage. Pour une photo par profil, c'est acceptable.
- **Effet sur le modèle :** `users.photo` reste un `TEXT` nullable contenant du base64 ; aucun fichier, aucune URL, aucun
  service de stockage objet.

## Alternatives écartées

- **Rester en PNG** : plus de poids pour le même contenu et le même caractère sans perte, sans autre avantage que
  l'absence de dépendance — ce qui n'est plus un argument décisif depuis qu'un encodeur Go pur existe.
- **JPEG** : compression avec perte, dégradation des aplats et des contours à 512 px, et qualité à calibrer puis à
  maintenir ; la reproductibilité du test devient dépendante de l'encodeur.
- **WebP avec perte** : même famille de format, mais réintroduit exactement le compromis qu'on refuse pour une donnée du
  profil.
- **Un encodeur lié à C (`libwebp`)** : performance supérieure, mais cgo, bibliothèque système et image de build plus
  lourde.
- **AVIF** : encodeur plus lourd encore, et aucun encodeur pure Go crédible.
- **Stocker l'image telle quelle, sans réduire** : poids non borné dans la base et dans chaque réponse `GET /profile`.
- **Stocker un fichier sur disque et une URL en base** : ajoute un cycle de vie de fichiers (nommage, sauvegarde,
  suppression) pour un seul champ, hors du cadre mono-fichier de l'ADR 0005.
