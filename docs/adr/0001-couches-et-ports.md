# ADR 0001 - Quatre couches en paquets Go, ports déclarés par le domaine

- **Statut :** accepté (révisé lors du portage en Go)
- **Date :** 2026-11-24

## Contexte

Le portage du service en Go remplace l'implémentation Python, désormais retirée du dépôt. Le découpage en couches
avait été décidé pour le code Python ; il est réexaminé ici pour vérifier qu'il survit au changement de langage et qu'il
se traduit en paquets Go nommés, importables et vérifiables.

## Décision

Quatre couches, un paquet Go par couche, avec des dépendances strictement orientées vers l'intérieur :

- `internal/domain` porte l'entité `Profil`, le type `Sexe`, les bornes et invariants (`Profil.Valider`), les erreurs
  métier et la **déclaration** du port de persistance (`internal/domain/ports.go`) ;
- `internal/application` contient un cas d'utilisation par fichier (`GererProfil`) et déclare ses ports secondaires
  (`NormaliseurPhoto`) ;
- `internal/infrastructure` fournit les adaptateurs, chacun dans son paquet : `sqlite` (schéma, dépôt, transactions) et
  `photos` (pipeline d'image) ;
- `internal/api` est l'adaptateur HTTP : routage, identité, décodage, correspondance erreur → code HTTP ;
- `cmd/wasalni/main.go` est l'unique racine de composition.

Le préfixe `internal/` interdit l'import de ces paquets depuis l'extérieur du module : la frontière ne dépend pas d'une
convention de nommage.

## Conséquences

- **Positif :** les règles métier et la persistance sont testables sans serveur HTTP ni fichier de base ; ajouter un UC
  revient à écrire un fichier dans `internal/application` et à réutiliser des ports ; les dépendances se vérifient à la
  lecture des imports ; le compilateur signale une dépendance interdite dès qu'elle est écrite.
- **Négatif :** une indirection de plus (interfaces, adaptateurs) et davantage de fichiers pour un petit système.
- **Sens des imports, vérifié :** `internal/api` n'importe que `internal/application` et `internal/domain`. La borne du
  corps HTTP lui appartient (`api.TailleMaxCorps`) et les erreurs du pipeline sont retraduites par le cas d'usage en
  `application.ErrPhotoInvalide` : aucune couche interne n'est connue de l'interface, et il n'y a pas deux sources de
  vérité.
- **Accepté parce que** le projet doit livrer 12 UC : le coût d'indirection est payé une fois, la structure sert ensuite.

## Alternatives écartées

- **Tout dans `main`** (un seul paquet, accès direct à SQLite) : couple le métier à la base et rend les tests d'UC
  dépendants d'un fichier.
- **Déclarer les ports dans l'infrastructure** : inverserait le sens des dépendances ; le domaine dépendrait de son
  adaptateur.
- **Conserver Python** : écarté par décision du porteur du projet (ADR 0009).
