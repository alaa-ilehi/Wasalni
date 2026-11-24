# ADR 0001 - Quatre couches et ports déclarés par le domaine

- **Statut :** accepté
- **Date :** 2026-11-24

## Contexte

L'implémentation livrée pour UC3 fonctionne, mais elle place dans `BookingService` des préoccupations de transport
et fait porter à chaque erreur métier un code HTTP. Onze cas d'utilisation restent à ajouter ; sans frontière nette,
chacun réinventera son accès aux données et son traitement d'erreur.

## Décision

Quatre couches (`api`, `application`, `infrastructure`, `domain`) avec des dépendances strictement orientées vers
l'intérieur :

- `domain` porte les entités, les invariants, les erreurs métier et la **déclaration** des ports (`app/domain/ports.py`) ;
- `application` contient un cas d'utilisation par fichier et n'utilise que les ports ;
- `infrastructure` fournit les adaptateurs (SQLite, notification, horloge) ;
- `api` est l'adaptateur HTTP ;
- `app/__init__.py` est l'unique racine de composition.

## Conséquences

- **Positif :** les règles métier et la concurrence sont testables sans Flask ni SQL ; ajouter un UC revient à écrire
  un fichier dans `application/` et à réutiliser des ports ; les dépendances se vérifient à la lecture des imports.
- **Négatif :** une indirection de plus (`Protocol`, adaptateurs) et davantage de fichiers pour un petit système.
- **Accepté parce que** le projet doit livrer 13 UC : le coût d'indirection est payé une fois, la structure sert ensuite.

## Alternatives écartées

- **Service unique accédant directement à SQLite** (statu quo) : couple le métier à la base, rend les tests d'UC
  dépendants d'un fichier de base.
- **Changer de framework** (FastAPI, Django) : rouvre tout le travail déjà validé pour un gain nul sur les critères du sujet.
