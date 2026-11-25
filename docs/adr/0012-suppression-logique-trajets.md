# ADR 0012 - Suppression logique des trajets

- **Statut :** appliqué — `rides.deleted_at` marque la suppression, les lectures publiques filtrent la colonne et les
  réservations actives sont annulées dans la même transaction
- **Date :** 2026-11-25

## Contexte

UC9 permet au conducteur de supprimer un trajet. Trois choses dépendent de la ligne `rides` après coup :

1. les réservations déjà prises, dont `bookings.ride_id` **référence** `rides(id)` (ADR 0004) ;
2. les évaluations, `ratings.ride_id` référençant la même ligne (ADR 0010) ;
3. l'historique des passagers qui consultent « mes réservations » (UC10) et voient le trajet associé.

Un `DELETE FROM rides` casserait les deux clés étrangères, ou exigerait de supprimer en cascade les réservations et
les avis — donc d'effacer des faits que les passagers ont vécus. La place rendue par la suppression doit par ailleurs
respecter l'invariant d'ADR 0004 : « places disponibles + réservations actives = places totales », y compris sur une
ligne supprimée.

## Décision

La suppression est **logique** : `rides.deleted_at` reçoit l'horodatage RFC 3339 UTC de la suppression, et la ligne
reste en base. Aucune autre table n'est retouchée autrement que par l'annulation décrite plus bas.

```sql
UPDATE rides
   SET seats_available = MIN(seats_available + ?, seats_total),
       deleted_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
 WHERE id = ?
```

`?` est le nombre de réservations effectivement annulées, lu sur `RowsAffected()` de l'`UPDATE` qui passe les
réservations `ACTIVE` du trajet à `CANCELLED`. Les deux écritures se font dans **une seule transaction** : soit le
trajet est supprimé et ses réservations annulées avec leurs places rendues, soit rien ne bouge. `MIN(...)` garde la
ligne cohérente même sur une base déjà incohérente, comme le fait l'annulation d'UC5.

La colonne est ajoutée par `ajouterColonneSupprime` : `PRAGMA table_info(rides)`, puis `ALTER TABLE rides ADD COLUMN
deleted_at TEXT` si elle manque. Le schéma reste idempotent et rejouable (ADR 0005), aucune table n'est reconstruite.

## Ce que la colonne change dans les lectures

Neuf conditions SQL la lisent, en trois rôles. La liste ci-dessous est exhaustive : elle se vérifie par
`grep -n "deleted_at" internal/infrastructure/sqlite/*.go | grep -v _test`.

### Trois lectures publiques excluent les trajets supprimés

| Fonction | Cas d'usage | Effet de la suppression |
|---|---|---|
| `Rechercher` | UC2, page de résultats paginée | le trajet disparaît des recherches |
| `ParIdentifiant` | UC7, et lectures préalables d'UC3, UC9 et UC11 | le trajet devient `404`, donc ni consultable ni modifiable |
| `ParConducteur` | UC10, « mes trajets » | le trajet disparaît de la liste du conducteur |

### Cinq conditions de garde-fou, à l'intérieur d'une transaction

| Fonction et condition | Cas d'usage | Sans la condition |
|---|---|---|
| `Reserver` : décrément de `seats_available` | UC3 | une place serait vendue sur un trajet supprimé |
| `Modifier` : lecture préalable | UC9 | un trajet supprimé serait modifiable |
| `Modifier` : `UPDATE ... WHERE ... deleted_at IS NULL` | UC9 | une suppression concurrente, entre la lecture et l'écriture, serait écrasée |
| `Supprimer` : lecture préalable | UC9 | une seconde suppression réussirait en silence |
| `Terminer` : lecture préalable | UC11 | un trajet supprimé pourrait être terminé |

La colonne du milieu n'est pas décorative : chaque condition ferme une course entre la vérification et l'écriture.

### Une jointure la tolère

`ReservationRepository.ParPassagerAvecTrajet` (UC10) porte la condition dans le `LEFT JOIN`, pas dans le `WHERE` :

```sql
LEFT JOIN rides r ON r.id = b.ride_id AND r.deleted_at IS NULL
```

Le trajet supprimé rend des colonnes nulles et la réservation reste dans le résultat, avec `Trajet` à `nil` côté
domaine, donc `ride: null` côté API. `ParPassager`, la même lecture sans jointure, ne filtre pas du tout : c'est la
raison d'être de la ligne conservée — l'historique du passager survit à la décision du conducteur.

Les deux index partiels `rides_search` et `rides_driver` portent la même condition `WHERE deleted_at IS NULL` : ils ne
contiennent que les trajets lisibles.

## Conséquences

- Une réservation déjà prise sur un trajet supprimé reste consultable, avec un trajet absent : l'API le dit
  explicitement (`ride: null`) plutôt que d'inventer un trajet fantôme.
- Une seconde suppression rend `404` (`ErrTrajetIntrouvable`) : la ligne reste, mais la clause `deleted_at IS NULL`
  de la lecture préalable la rend invisible, donc la suppression n'est pas rejouable en silence.
- La ligne conservée garde son `seats_available` remis à niveau : rouvrir un trajet un jour ne demanderait qu'un
  `UPDATE deleted_at = NULL`, sans recalcul de places.
- La table grossit des trajets supprimés. À cette échelle (un trajet est une ligne de quelques centaines d'octets),
  c'est négligeable ; si le volume devenait un problème, une purge des lignes supprimées et sans réservation serait la
  suite naturelle — elle devrait alors traiter explicitement `bookings` et `ratings`.

## Alternatives écartées

- **`DELETE FROM rides`** : casse `bookings.ride_id` et `ratings.ride_id`, ou impose une cascade qui efface
  l'historique des passagers et les évaluations reçues.
- **Colonne `deleted BOOLEAN`** : ne dit pas *quand*, alors que l'horodatage sert au diagnostic et à une éventuelle
  purge.
- **Table d'archive `rides_supprimes`** : recopie et double le schéma pour un besoin qui n'existe pas encore.
