#!/usr/bin/env bash
# TP1 Partie 1 (Elaboration), steps 1 to 4, automated with git + GitHub CLI.
#   1. repo, board, UC tickets, 4 milestones, one branch per UC, phase tags
#   2. PlantUML diagrams committed
#   3. baseline architecture (UC3 end to end) + first test passing in CI
#   4. tag v0.1-elaboration + close the Elaboration milestone
#
# Prerequisites: git, GitHub CLI (`gh auth login`; for the board also `gh auth refresh -s project`).
# Usage: ./scripts/elaboration.sh [repo-name]
set -euo pipefail
REPO_NAME="${1:-carpooling}"

command -v gh >/dev/null || { echo "Install the GitHub CLI first: https://cli.github.com"; exit 1; }
gh auth status >/dev/null

echo "== 0. Local check =="
pip install -r requirements.txt -q
pytest -q

echo "== 1a. Inception commit on main (vision + use case diagram) =="
[ -d .git ] || git init -b main
cp README.md /tmp/README.full.md
printf '# Carpooling API\n\nIntercity carpooling API (TP1). See docs/vision.md.\n' > README.md
git add README.md .gitignore docs/vision.md docs/uml/usecases.puml
git commit -m "docs: vision, actors, use cases and use case diagram (Inception)"
git tag -a v0.0-inception -m "Inception: vision, UCs, priorities, plan"

echo "== 1b. GitHub repo =="
gh repo create "$REPO_NAME" --public --source=. --remote=origin --push
git push origin v0.0-inception
R=$(gh repo view --json nameWithOwner -q .nameWithOwner)

echo "== 1c. Milestones = the 4 phases =="
for m in Inception Elaboration Construction Transition; do
  gh api "repos/$R/milestones" -f title="$m" >/dev/null && echo "  $m"
done
gh api -X PATCH "repos/$R/milestones/$(gh api "repos/$R/milestones" --jq '.[] | select(.title=="Inception") | .number')" \
  -f state=closed >/dev/null     # Inception is already done (tag v0.0-inception)

echo "== 1d. One ticket per UC =="
ticket() {  # title milestone body -> prints issue number
  gh issue create --title "$1" --milestone "$2" --body "$3" | grep -o '[0-9]*$'
}
N3=$(ticket "UC3 Book a seat (technical risk)"          Elaboration  "docs/uc3-book-seat.md - code porté par la branche architecture-elaboration")
N1=$(ticket "UC1 Post a ride"                           Construction "branch uc1-post-ride")
N2=$(ticket "UC2 Search rides"                         Construction "branch uc2-search-rides")
N7=$(ticket "UC7 View ride details"                    Construction "branch uc7-ride-details")
N8=$(ticket "UC8 Manage profile"                       Construction "branch uc8-manage-profile")
N13=$(ticket "UC13 Authenticate (external system)"      Construction "branch uc13-authenticate")
N5=$(ticket "UC5 Cancel a booking"                     Transition   "branch uc5-cancel-booking")
N9=$(ticket "UC9 Edit or delete a published ride"      Transition   "branch uc9-edit-ride")
N10=$(ticket "UC10 View my rides / bookings"            Transition   "branch uc10-my-rides")
N11=$(ticket "UC11 Complete a ride"                     Transition   "branch uc11-complete-ride")
N12=$(ticket "UC12 Receive notifications (external)"    Transition   "branch uc12-notifications")
N6=$(ticket "UC6 Rate each other"                      Transition   "branch uc6-rate-each-other")
UC_TICKETS="$N3 $N1 $N2 $N7 $N8 $N13 $N5 $N9 $N10 $N11 $N12 $N6"
echo "  UC3=#$N3 UC1=#$N1 UC2=#$N2 UC7=#$N7 UC8=#$N8 UC13=#$N13"
echo "  UC5=#$N5 UC9=#$N9 UC10=#$N10 UC11=#$N11 UC12=#$N12 UC6=#$N6"

echo "== 1e. Project board (needs the 'project' scope) =="
if P=$(gh project create --owner "@me" --title "$REPO_NAME board" --format json -q .number 2>/dev/null); then
  for n in $UC_TICKETS; do
    gh project item-add "$P" --owner "@me" --url "https://github.com/$R/issues/$n" >/dev/null
  done
  gh project link "$P" --owner "@me" --repo "$R" >/dev/null 2>&1 || true
  echo "  board #$P created with the 12 tickets"
else
  echo "  Could not create the board. Run: gh auth refresh -s project   (or create it in the web UI:"
  echo "  Projects -> New project -> Board, then add the 12 issues)"
fi

echo "== 2-3. Branch architecture-elaboration: diagrams + baseline architecture + first tests =="
git checkout -b architecture-elaboration
cp /tmp/README.full.md README.md
git add README.md docs/uc3-book-seat.md docs/uml/uc3-sequence.puml docs/uml/uc3-class.puml
git commit -m "docs(uc3): UC sheet, class and sequence diagrams (#$N3)"
git add app/errors.py app/db.py app/models.py app/__init__.py run.py requirements.txt
git commit -m "feat(uc3): application skeleton, SQLite schema, error model (#$N3)"
git add app/repositories app/services app/controllers
git commit -m "feat(uc3): book-a-seat route end to end: controller, service, repository (#$N3)"
git add tests
git commit -m "test(uc3): unit, integration and concurrency tests (#$N3)"
git add .github/workflows/ci.yml
git commit -m "ci: run pytest on every push and pull request"
git add scripts/elaboration.sh
git commit -m "chore: add elaboration automation script"
git push -u origin architecture-elaboration

gh pr create --base main --head architecture-elaboration \
  --title "UC3 Book a seat (baseline architecture)" \
  --body "Closes #$N3

Baseline architecture running end to end for the risky UC: controller -> service -> repository -> SQLite.
- atomic seat decrement (conditional UPDATE in one transaction)
- unit tests, integration test of the nominal scenario, concurrency test
- CI must be green before merge"

echo "   waiting for CI..."
sleep 15
gh pr checks architecture-elaboration --watch --interval 10        # exits non-zero if CI fails -> script stops
gh pr merge architecture-elaboration --merge --delete-branch

echo "== 4. Tag v0.1-elaboration and close the milestone =="
git checkout main
git pull origin main
git tag -a v0.1-elaboration -m "Baseline architecture running: UC3 end to end, CI green"
git push origin v0.1-elaboration
gh api -X PATCH "repos/$R/milestones/$(gh api "repos/$R/milestones?state=all" --jq '.[] | select(.title=="Elaboration") | .number')" \
  -f state=closed >/dev/null
echo "  milestone Elaboration closed"

echo "== One branch per remaining UC =="
for b in uc1-post-ride uc2-search-rides uc5-cancel-booking uc6-rate-each-other \
         uc7-ride-details uc8-manage-profile uc9-edit-ride uc10-my-rides \
         uc11-complete-ride uc12-notifications uc13-authenticate; do
  git branch "$b" main
  git push origin "$b"
done

echo
echo "Done. Tags: $(git tag | tr '\n' ' ')"
echo "Next (Construction): git checkout uc1-post-ride, add UC1, open a PR with 'Closes #$N1'."
