# Elaboration without the script (manual commands)

## 1. Repo, board, tickets, milestones, branches, tags
```bash
git init -b main
git add README.md .gitignore docs/vision.md docs/uml/usecases.puml
git commit -m "docs: vision, actors, use cases and use case diagram (Inception)"
git tag -a v0.0-inception -m "Inception"
# create an empty repo on github.com, then:
git remote add origin https://github.com/<you>/carpooling.git
git push -u origin main && git push origin v0.0-inception
```
On GitHub:
- Issues -> Milestones: create Inception (close it), Elaboration, Construction, Transition.
- Issues -> New issue, one per UC, with its milestone: UC3 (Elaboration); UC4, UC1, UC2 (Construction); UC5, UC6 (Transition).
- Projects -> New project -> Board -> add the 6 issues.

## 2-3. Diagrams and baseline on the UC3 branch
```bash
git checkout -b uc3-book-seat
git add docs/uc3-book-seat.md docs/uml && git commit -m "docs(uc3): UC sheet, class and sequence diagrams (#1)"
git add app/errors.py app/db.py app/models.py app/__init__.py run.py requirements.txt && git commit -m "feat(uc3): skeleton (#1)"
git add app/repositories app/services app/controllers && git commit -m "feat(uc3): book-a-seat route end to end (#1)"
git add tests && git commit -m "test(uc3): unit, integration, concurrency (#1)"
git add .github && git commit -m "ci: run pytest"
git push -u origin uc3-book-seat
```
Open a Pull Request (body: `Closes #1`), wait for the green check, click Merge.

## 4. Tag and close
```bash
git checkout main && git pull
git tag -a v0.1-elaboration -m "Baseline architecture running"
git push origin v0.1-elaboration
```
Close the Elaboration milestone (Issues -> Milestones -> Close).

## One branch per remaining UC
```bash
for b in uc4-pay-booking uc1-post-ride uc2-search-rides uc5-cancel-booking uc6-rate-driver; do
  git branch $b main && git push origin $b; done
```
