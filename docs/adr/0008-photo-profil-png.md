# ADR 0008 - Photo de profil en PNG sans perte

- **Statut :** accepté
- **Date :** 2026-11-24

## Contexte

UC8 accepte une photo de profil. Le contrat HTTP la reçoit en base64 dans le corps de la requête, et la base la stocke en
texte : sans borne, une photo peut faire plusieurs mégaoctets et alourdir chaque lecture de profil.

Le service doit rester à la bibliothèque standard plus `modernc.org/sqlite` (ADR 0009) : aucun codec supplémentaire ne peut
être ajouté. Il faut donc choisir un format que la bibliothèque standard sait encoder, et une politique de réduction qui
tienne dans une taille connue.

## Décision

Le pipeline vit dans `internal/infrastructure/photos` (`Normaliser`) :

- **PNG sans perte plutôt que JPEG.** Le format retenu est PNG, encodé avec `png.BestCompression`. La photo est une donnée
  du profil utilisateur : une compression avec perte introduirait une dégradation non réversible et variable selon le
  contenu, difficile à expliquer à l'utilisateur comme à tester.
- **Côté maximal : 512 px.** Le plus grand côté est ramené à 512 px au plus par moyenne de blocs (box filter) ; le
  redimensionneur est écrit à la main (`internal/infrastructure/photos/couleur.go`), ce qui évite d'ajouter une
  bibliothèque de rééchantillonnage.
- **Poids maximal : 512 Kio.** Si le PNG dépasse 512 Kio, le côté est réduit par paliers de 64 px — 512, 448, 384, 320,
  256, 192, 128 — et l'image est réencodée à chaque essai.
- **Refus seulement si c'est impossible.** L'image n'est refusée (`ErrImageTropLourde`, `400`) qu'à 128 px si le PNG
  dépasse encore 512 Kio. Une image illisible ou d'un format non reconnu donne `ErrImageIllisible` (`400`).
- **Entrée bornée avant décodage.** Le corps de la requête est limité à 10 Mio (`photos.TailleMaxCorps`) par
  `http.MaxBytesReader` ; au-delà, `413`. Le préfixe d'une data URL (`data:image/…;base64,`) est accepté, ainsi que les
  retours à la ligne.
- **Recadrage côté navigateur prévu plus tard.** Le service ne recadre pas et ne détecte pas de visage : il réduit et
  encode. Le cadrage final appartiendra à l'interface web, hors périmètre aujourd'hui.

## Conséquences

- **Positif :** aucune perte de qualité, résultat déterministe pour une même entrée, poids stocké borné et connu ; aucune
  dépendance de codec ; PNG et JPEG en entrée sont acceptés grâce aux décodeurs de la bibliothèque standard.
- **Négatif :** à qualité visuelle comparable, un PNG pèse plus qu'un JPEG : les paliers de réduction se déclenchent plus
  souvent et les photos stockées sont plus petites que ce qu'un JPEG aurait permis. Le box filter coûte du CPU, et le
  nombre de pixels décodés n'est borné que par la limite de 10 Mio de l'entrée. Le base64 ajoute environ un tiers de
  surcoût au stockage.
- **Effet sur le modèle :** `users.photo` reste un `TEXT` nullable contenant du base64 ; aucun fichier, aucune URL, aucun
  service de stockage objet.

## Alternatives écartées

- **JPEG** : compression avec perte, dégradation des aplats et des contours à 512 px, et qualité à calibrer puis à
  maintenir ; la reproductibilité du test devient dépendante de l'encodeur.
- **WebP** : pas d'encodeur dans la bibliothèque standard ; il faudrait `golang.org/x/image` ou une bibliothèque liée à C,
  donc une dépendance et, dans le second cas, cgo.
- **AVIF** : même raison, avec un encodeur plus lourd encore.
- **Stocker l'image telle quelle, sans réduire** : poids non borné dans la base et dans chaque réponse `GET /profile`.
- **Stocker un fichier sur disque et une URL en base** : ajoute un cycle de vie de fichiers (nommage, sauvegarde,
  suppression) pour un seul champ, hors du cadre mono-fichier de l'ADR 0005.
