// Wasalni : interface minimale en HTML/CSS/JS, sans framework. Elle appelle l'API servie sur la même origine.

const $ = (sel) => document.querySelector(sel);
const session = {
  get token() { return localStorage.getItem("wasalni_token"); },
  get id() { return localStorage.getItem("wasalni_id"); },
  save(id, token) { localStorage.setItem("wasalni_id", id); localStorage.setItem("wasalni_token", token); },
  clear() { localStorage.removeItem("wasalni_id"); localStorage.removeItem("wasalni_token"); },
};

// ---------- utilitaires ----------

async function api(method, path, body) {
  const headers = {};
  if (session.token) headers["Authorization"] = "Bearer " + session.token;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const res = await fetch(path, { method, headers, body: body !== undefined ? JSON.stringify(body) : undefined });
  const text = await res.text();
  const data = text ? JSON.parse(text) : null;
  if (res.status === 401 && session.token && !path.startsWith("/auth/")) {
    session.clear();
    render();
  }
  if (!res.ok) throw new Error((data && data.error) || "Erreur " + res.status);
  return data;
}

function showMessage(text, ok = true) {
  const box = $("#message");
  box.textContent = text;
  box.className = ok ? "ok" : "err";
  box.hidden = false;
  clearTimeout(showMessage.timer);
  showMessage.timer = setTimeout(() => (box.hidden = true), 4000);
}

// Exécute une action et affiche l'erreur éventuelle.
async function run(fn, successText) {
  try {
    await fn();
    if (successText) showMessage(successText);
  } catch (e) {
    showMessage(e.message, false);
  }
}

function esc(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

function fmtDate(iso) {
  return new Date(iso).toLocaleString("fr-FR", { dateStyle: "medium", timeStyle: "short" });
}

// datetime-local → RFC3339 (UTC, sans millisecondes)
function toRFC3339(local) {
  return new Date(local).toISOString().replace(/\.\d{3}Z$/, "Z");
}

// RFC3339 → valeur pour un champ datetime-local
function toLocalInput(iso) {
  const d = new Date(iso);
  const pad = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function photoSrc(photo) {
  return photo.startsWith("data:") ? photo : "data:image/webp;base64," + photo;
}

function rideSummary(r) {
  return `
    <h3>${esc(r.origin)} → ${esc(r.destination)} <span class="badge ${esc(r.status)}">${esc(r.status)}</span></h3>
    <p>Départ : ${esc(fmtDate(r.departure_time))}</p>
    <p>Places : ${r.seats_available} / ${r.seats_total} — Prix : ${r.price} DT — Annulation jusqu'à ${r.cancel_delay_hours} h avant</p>
    <p>Conducteur : ${esc(r.driver_id)}</p>`;
}

// ---------- affichage général ----------

function render() {
  const logged = !!session.token;
  $("#auth-view").hidden = logged;
  $("#app-view").hidden = !logged;
  $("#session-info").hidden = !logged;
  $("#user-id").textContent = session.id || "";
  if (logged) openTab(currentTab);
}

let currentTab = "search";
function openTab(name) {
  currentTab = name;
  document.querySelectorAll("nav button").forEach((b) => b.classList.toggle("active", b.dataset.tab === name));
  document.querySelectorAll(".tab").forEach((s) => (s.hidden = s.id !== "tab-" + name));
  if (name === "my-rides") loadMyRides();
  if (name === "my-bookings") loadMyBookings();
  if (name === "profile") loadProfile();
  if (name === "search") search();
}
document.querySelectorAll("nav button").forEach((b) => b.addEventListener("click", () => openTab(b.dataset.tab)));

// ---------- UC13 : authentification ----------

$("#form-auth").addEventListener("submit", (e) => {
  e.preventDefault();
  const action = e.submitter.dataset.action;
  const f = e.target;
  run(async () => {
    const data = await api("POST", "/auth/" + action, { id: f.id.value.trim(), password: f.password.value });
    session.save(data.id, data.token);
    f.reset();
    render();
  }, action === "register" ? "Compte créé. Pensez à compléter votre profil." : "Bienvenue !");
});

$("#btn-logout").addEventListener("click", () =>
  run(async () => {
    try { await api("DELETE", "/auth/session"); } finally { session.clear(); render(); }
  }, "Déconnecté."));

// ---------- UC2 : recherche (avec pagination) ----------

let nextCursor = null;

async function search(append = false) {
  const f = $("#form-search");
  const params = new URLSearchParams();
  for (const k of ["origin", "destination", "date"]) if (f[k].value) params.set(k, f[k].value);
  params.set("limit", "10");
  if (append && nextCursor) params.set("after", nextCursor);
  await run(async () => {
    const page = await api("GET", "/rides?" + params);
    nextCursor = page.next_cursor;
    const html = page.rides.map((r) => `
      <div class="card">${rideSummary(r)}
        <div class="row">
          <button data-details="${esc(r.id)}" class="secondary">Détails</button>
          ${r.driver_id !== session.id && r.seats_available > 0 && r.status === "PUBLISHED"
            ? `<button data-book="${esc(r.id)}">Réserver</button>` : ""}
        </div>
      </div>`).join("");
    const box = $("#search-results");
    if (append) box.insertAdjacentHTML("beforeend", html);
    else box.innerHTML = html || '<p class="empty">Aucun trajet trouvé.</p>';
    $("#btn-more").hidden = !nextCursor;
  });
}

$("#form-search").addEventListener("submit", (e) => { e.preventDefault(); search(); });
$("#btn-more").addEventListener("click", () => search(true));

// Délégation : boutons Détails / Réserver dans les résultats
$("#search-results").addEventListener("click", (e) => {
  const details = e.target.dataset.details;
  const book = e.target.dataset.book;
  if (details) showDetails(details);
  if (book) run(async () => { await api("POST", `/rides/${book}/bookings`); await search(); }, "Place réservée !");
});

// ---------- UC7 : détails ----------

async function showDetails(id) {
  await run(async () => {
    const r = await api("GET", "/rides/" + encodeURIComponent(id));
    let driver = '<p class="empty">Le conducteur n\'a pas encore de profil.</p>';
    if (r.driver) {
      driver = `
        <h4>Conducteur</h4>
        ${r.driver.photo ? `<img src="${photoSrc(r.driver.photo)}" alt="" style="width:64px;height:64px;border-radius:50%">` : ""}
        <p>${esc(r.driver.name)}, ${r.driver.age} ans ${r.driver.sex ? "(" + esc(r.driver.sex) + ")" : ""}</p>
        <p>Note : ${r.driver.rating == null ? "pas encore évalué" : r.driver.rating.toFixed(1) + " / 5"}</p>`;
    }
    $("#details-body").innerHTML = `<div>${rideSummary(r)}</div>${driver}`;
    $("#dlg-details").showModal();
  });
}

// ---------- UC1 / UC9 : publier ou modifier ----------

$("#form-ride").addEventListener("submit", (e) => {
  e.preventDefault();
  const f = e.target;
  const body = {
    origin: f.origin.value.trim(),
    destination: f.destination.value.trim(),
    departure_time: toRFC3339(f.departure_time.value),
    seats_total: Number(f.seats_total.value),
    price: Number(f.price.value),
  };
  if (f.cancel_delay_hours.value !== "") body.cancel_delay_hours = Number(f.cancel_delay_hours.value);
  const editing = f.ride_id.value;
  run(async () => {
    if (editing) await api("PUT", "/rides/" + encodeURIComponent(editing), body);
    else await api("POST", "/rides", body);
    resetRideForm();
    openTab("my-rides");
  }, editing ? "Trajet modifié." : "Trajet publié !");
});

function resetRideForm() {
  const f = $("#form-ride");
  f.reset();
  f.ride_id.value = "";
  $("#publish-title").textContent = "Publier un trajet";
  $("#btn-ride-submit").textContent = "Publier";
  $("#btn-ride-cancel").hidden = true;
}
$("#btn-ride-cancel").addEventListener("click", resetRideForm);

function editRide(r) {
  const f = $("#form-ride");
  f.ride_id.value = r.id;
  f.origin.value = r.origin;
  f.destination.value = r.destination;
  f.departure_time.value = toLocalInput(r.departure_time);
  f.seats_total.value = r.seats_total;
  f.price.value = r.price;
  f.cancel_delay_hours.value = r.cancel_delay_hours;
  $("#publish-title").textContent = "Modifier le trajet";
  $("#btn-ride-submit").textContent = "Enregistrer";
  $("#btn-ride-cancel").hidden = false;
  openTab("publish");
}

// ---------- UC10 : mes trajets ----------

let myRides = [];
async function loadMyRides() {
  await run(async () => {
    myRides = await api("GET", "/me/rides");
    $("#my-rides").innerHTML = myRides.map((r) => `
      <div class="card">${rideSummary(r)}
        <div class="row">
          ${r.status === "PUBLISHED" ? `
            <button data-edit="${esc(r.id)}" class="secondary">Modifier</button>
            <button data-complete="${esc(r.id)}">Terminer</button>
            <button data-delete="${esc(r.id)}" class="danger">Supprimer</button>` : ""}
          ${r.status === "COMPLETED" ? `<button data-rate="${esc(r.id)}">Évaluer un passager</button>` : ""}
        </div>
      </div>`).join("") || '<p class="empty">Vous n\'avez publié aucun trajet.</p>';
  });
}

$("#my-rides").addEventListener("click", (e) => {
  const d = e.target.dataset;
  if (d.edit) editRide(myRides.find((r) => r.id === d.edit));
  if (d.complete) run(async () => { await api("POST", `/rides/${d.complete}/complete`); await loadMyRides(); }, "Trajet terminé.");
  if (d.delete && confirm("Supprimer ce trajet ? Les réservations seront annulées."))
    run(async () => { await api("DELETE", "/rides/" + d.delete); await loadMyRides(); }, "Trajet supprimé.");
  if (d.rate) openRate(d.rate, "");
});

// ---------- UC10 : mes réservations (UC5 annuler, UC6 évaluer) ----------

async function loadMyBookings() {
  await run(async () => {
    const list = await api("GET", "/me/bookings");
    $("#my-bookings").innerHTML = list.map((b) => `
      <div class="card">
        ${b.ride ? rideSummary(b.ride) : "<h3>Trajet supprimé</h3>"}
        <p>Réservation : <span class="badge ${esc(b.status)}">${esc(b.status)}</span> — le ${esc(fmtDate(b.created_at))}</p>
        <div class="row">
          ${b.status === "ACTIVE" && b.ride && b.ride.status === "PUBLISHED"
            ? `<button data-cancel="${esc(b.id)}" class="danger">Annuler la réservation</button>` : ""}
          ${b.ride && b.ride.status === "COMPLETED" && b.status === "ACTIVE"
            ? `<button data-rate="${esc(b.ride.id)}" data-target="${esc(b.ride.driver_id)}">Évaluer le conducteur</button>` : ""}
        </div>
      </div>`).join("") || '<p class="empty">Aucune réservation.</p>';
  });
}

$("#my-bookings").addEventListener("click", (e) => {
  const d = e.target.dataset;
  if (d.cancel) run(async () => { await api("DELETE", "/bookings/" + d.cancel); await loadMyBookings(); }, "Réservation annulée.");
  if (d.rate) openRate(d.rate, d.target);
});

// ---------- UC6 : évaluation ----------

function openRate(rideId, target) {
  const f = $("#form-rate");
  f.reset();
  f.ride_id.value = rideId;
  f.target.value = target || "";
  f.target.readOnly = !!target;
  $("#dlg-rate").showModal();
}
$("#btn-rate-close").addEventListener("click", () => $("#dlg-rate").close());

$("#form-rate").addEventListener("submit", (e) => {
  e.preventDefault();
  const f = e.target;
  run(async () => {
    await api("POST", `/rides/${f.ride_id.value}/ratings`, {
      target: f.target.value.trim(),
      score: Number(f.score.value),
      comment: f.comment.value.trim(),
    });
    $("#dlg-rate").close();
  }, "Évaluation enregistrée. Merci !");
});

// ---------- UC8 : profil ----------

async function loadProfile() {
  const f = $("#form-profile");
  const img = $("#profile-photo");
  try {
    const p = await api("GET", "/profile");
    f.name.value = p.name;
    f.age.value = p.age;
    f.sex.value = p.sex || "";
    img.hidden = !p.photo;
    if (p.photo) img.src = photoSrc(p.photo);
  } catch (e) {
    // 404 : pas encore de profil, le formulaire reste vide.
    img.hidden = true;
  }
}

function readFileAsDataURL(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result);
    reader.onerror = reject;
    reader.readAsDataURL(file);
  });
}

$("#form-profile").addEventListener("submit", (e) => {
  e.preventDefault();
  const f = e.target;
  run(async () => {
    const body = { name: f.name.value.trim(), age: Number(f.age.value) };
    if (f.sex.value) body.sex = f.sex.value;
    if (f.photo.files[0]) body.photo = await readFileAsDataURL(f.photo.files[0]);
    await api("PUT", "/profile", body);
    f.photo.value = "";
    await loadProfile();
  }, "Profil enregistré.");
});

render();
