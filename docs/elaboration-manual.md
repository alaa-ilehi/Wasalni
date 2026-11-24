# Élaboration sans le script (commandes à la main)

Mêmes étapes que `scripts/elaboration.sh`, exécutées une par une.

## 1. Dépôt, tableau de bord, tickets, milestones, branches, tags
```bash
git init -b main
git add README.md .gitignore docs/vision.md docs/use-case-elaboration.md docs/uml/usecases.puml docs/uml/usecases.png
git commit -m "docs: vision, acteurs, cas d'utilisation et diagramme (Inception)"
git tag -a v0.0-inception -m "Inception"
# créer un dépôt vide sur github.com, puis :
git remote add origin https://github.com/<vous>/carpooling.git
git push -u origin main && git push origin v0.0-inception
```
Sur GitHub :
- Issues -> Milestones : créer Inception (la fermer), Elaboration, Construction, Transition.
- Issues -> New issue, un ticket par UC, avec son milestone : UC3 (Elaboration) ;
  UC1, UC2, UC7, UC8, UC13 (Construction) ; UC5, UC6, UC9, UC10, UC11, UC12 (Transition).
- Projects -> New project -> Board -> ajouter les 12 tickets.
- Une branche par UC, nommée `ucN-<slug>` (voir le tableau du `README.md`) ; le tableau de bord
  peut suivre une colonne par branche.

## 2-3. Diagrammes et architecture de base sur la branche `architecture-elaboration`
```bash
git checkout -b architecture-elaboration
git add docs/uc3-book-seat.md docs/uml && git commit -m "docs(uc3): fiche UC, diagrammes de classe et de séquence (#1)"
git add app/errors.py app/db.py app/models.py app/__init__.py run.py requirements.txt && git commit -m "feat(uc3): squelette applicatif et schéma SQLite (#1)"
git add app/repositories app/services app/controllers && git commit -m "feat(uc3): route de réservation de bout en bout (#1)"
git add tests && git commit -m "test(uc3): tests unitaires, intégration, concurrence (#1)"
git add .github && git commit -m "ci: lancer pytest"
git push -u origin architecture-elaboration
```
Ouvrir une Pull Request (`Closes #1`), attendre le check vert, puis merger.

## 4. Tag et fermeture du jalon
```bash
git checkout main && git pull
git tag -a v0.1-elaboration -m "Architecture de base opérationnelle"
git push origin v0.1-elaboration
```
Fermer le milestone Elaboration (Issues -> Milestones -> Close).

## Une branche par UC restant
```bash
for b in uc1-post-ride uc2-search-rides uc5-cancel-booking uc6-rate-each-other \
         uc7-ride-details uc8-manage-profile uc9-edit-ride uc10-my-rides \
         uc11-complete-ride uc12-notifications uc13-authenticate; do
  git branch $b main && git push origin $b; done
```
