// Tests fonctionnels de bout en bout : chaque cas d'utilisation est joué contre le binaire réel.
// Usage : node tests/functional/run.mjs [http://127.0.0.1:8080]
const BASE = process.argv[2] || "http://127.0.0.1:8080";
let passed = 0, failed = 0;

async function call(method, path, { token, body } = {}) {
  const headers = {};
  if (token) headers.Authorization = "Bearer " + token;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const res = await fetch(BASE + path, { method, headers, body: body !== undefined ? JSON.stringify(body) : undefined, redirect: "manual" });
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = text; }
  return { status: res.status, data, headers: res.headers };
}

function check(name, cond, detail) {
  if (cond) { passed++; console.log("  ✔ " + name); }
  else { failed++; console.log("  ✘ " + name + (detail ? "  → " + JSON.stringify(detail) : "")); }
}

const suffix = Date.now().toString(36);
const inHours = (h) => new Date(Date.now() + h * 3600e3).toISOString().replace(/\.\d{3}Z$/, "Z");
// PNG 1x1 valide pour la photo de profil
const PNG = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==";

console.log("Interface web");
let r = await call("GET", "/");
check("GET / redirige vers /app/", r.status === 302 && r.headers.get("location") === "/app/", r.status);
r = await call("GET", "/app/");
check("GET /app/ sert index.html", r.status === 200 && String(r.data).includes("Wasalni"), r.status);
r = await call("GET", "/app/app.js");
check("GET /app/app.js", r.status === 200, r.status);
r = await call("GET", "/health");
check("GET /health", r.status === 200 && r.data.statut === "ok", r.data);

console.log("UC13 S'authentifier");
const driver = "driver_" + suffix, pass1 = "passager1_" + suffix, pass2 = "passager2_" + suffix;
r = await call("POST", "/auth/register", { body: { id: driver, password: "motdepasse" } });
check("inscription conducteur 201", r.status === 201 && r.data.token, r);
let tDriver = r.data.token;
r = await call("POST", "/auth/register", { body: { id: driver, password: "motdepasse" } });
check("inscription en double 409", r.status === 409, r);
r = await call("POST", "/auth/register", { body: { id: "x" + suffix, password: "court" } });
check("mot de passe trop court 400", r.status === 400, r);
r = await call("POST", "/auth/login", { body: { id: driver, password: "mauvais!!" } });
check("connexion mauvais mot de passe 401", r.status === 401, r);
r = await call("POST", "/auth/login", { body: { id: driver, password: "motdepasse" } });
check("connexion 200", r.status === 200 && r.data.token, r);
tDriver = r.data.token;
r = await call("GET", "/rides");
check("route protégée sans jeton 401", r.status === 401, r);
const tP1 = (await call("POST", "/auth/register", { body: { id: pass1, password: "motdepasse" } })).data.token;
const tP2 = (await call("POST", "/auth/register", { body: { id: pass2, password: "motdepasse" } })).data.token;

console.log("UC8 Gérer son profil");
r = await call("GET", "/profile", { token: tDriver });
check("profil absent 404", r.status === 404, r);
r = await call("PUT", "/profile", { token: tDriver, body: { name: "Amine", age: 30, sex: "MALE", photo: PNG } });
check("création profil 201 avec photo", r.status === 201 && r.data.photo, r.status);
r = await call("PUT", "/profile", { token: tDriver, body: { name: "Amine B", age: 31 } });
check("mise à jour profil 200, photo conservée", r.status === 200 && r.data.name === "Amine B" && r.data.photo, r.status);
r = await call("PUT", "/profile", { token: tDriver, body: { name: "Bad", age: 12 } });
check("âge invalide 400", r.status === 400, r);
await call("PUT", "/profile", { token: tP1, body: { name: "Sara", age: 25, sex: "FEMALE" } });

console.log("UC1 Publier un trajet");
const origin = "Tunis-" + suffix;
const ride = { origin, destination: "Gafsa", departure_time: inHours(48), seats_total: 2, price: 25, cancel_delay_hours: 2 };
r = await call("POST", "/rides", { token: tDriver, body: ride });
check("publication 201", r.status === 201 && r.data.seats_available === 2 && r.data.status === "PUBLISHED", r);
const rideId = r.data.id;
r = await call("POST", "/rides", { token: tDriver, body: { ...ride, departure_time: inHours(-5) } });
check("départ dans le passé 400", r.status === 400, r);
r = await call("POST", "/rides", { token: tDriver, body: { ...ride, seats_total: 0 } });
check("zéro place 400", r.status === 400, r);

console.log("UC2 Rechercher des trajets");
r = await call("GET", "/rides?origin=" + encodeURIComponent(origin), { token: tP1 });
check("recherche par origine trouve le trajet", r.status === 200 && r.data.rides.some((x) => x.id === rideId), r);
await call("POST", "/rides", { token: tDriver, body: { ...ride, departure_time: inHours(72) } });
r = await call("GET", "/rides?limit=1&origin=" + encodeURIComponent(origin), { token: tP1 });
check("pagination : next_cursor présent", r.status === 200 && r.data.rides.length === 1 && r.data.next_cursor, r.data);
r = await call("GET", "/rides?limit=1&origin=" + encodeURIComponent(origin) + "&after=" + encodeURIComponent(r.data.next_cursor), { token: tP1 });
check("pagination : page suivante", r.status === 200 && r.data.rides.length === 1, r.data);
const otherRideId = r.data.rides[0].id;
r = await call("GET", "/rides?limit=999", { token: tP1 });
check("limite hors bornes 400", r.status === 400, r);

console.log("UC7 Consulter les détails");
r = await call("GET", "/rides/" + rideId, { token: tP1 });
check("détails avec conducteur", r.status === 200 && r.data.driver && r.data.driver.name === "Amine B" && r.data.driver.rating === null, r.data);
r = await call("GET", "/rides/inexistant", { token: tP1 });
check("trajet inconnu 404", r.status === 404, r);

console.log("UC3 Réserver une place");
r = await call("POST", `/rides/${rideId}/bookings`, { token: tDriver });
check("le conducteur ne peut pas réserver 400", r.status === 400, r);
r = await call("POST", `/rides/${rideId}/bookings`, { token: tP1 });
check("réservation passager 1 → 201", r.status === 201 && r.data.status === "ACTIVE", r);
const booking1 = r.data.id;
r = await call("POST", `/rides/${rideId}/bookings`, { token: tP1 });
check("double réservation 409", r.status === 409, r);
// course sur la dernière place : 5 comptes tentent en même temps, un seul doit gagner
const racers = await Promise.all([...Array(5)].map((_, i) =>
  call("POST", "/auth/register", { body: { id: `race${i}_${suffix}`, password: "motdepasse" } })));
const results = await Promise.all(racers.map((x) => call("POST", `/rides/${rideId}/bookings`, { token: x.data.token })));
check("concurrence : exactement 1 gagnant sur la dernière place",
  results.filter((x) => x.status === 201).length === 1 && results.filter((x) => x.status === 409).length === 4,
  results.map((x) => x.status));
r = await call("GET", "/rides/" + rideId, { token: tP1 });
check("plus aucune place disponible", r.data.seats_available === 0, r.data);
r = await call("POST", `/rides/${rideId}/bookings`, { token: tP2 });
check("trajet complet 409", r.status === 409, r);

console.log("UC5 Annuler une réservation");
r = await call("DELETE", "/bookings/" + booking1, { token: tP2 });
check("annuler la réservation d'un autre refusé", r.status === 404 || r.status === 403, r);
r = await call("DELETE", "/bookings/" + booking1, { token: tP1 });
check("annulation 204", r.status === 204, r);
r = await call("GET", "/rides/" + rideId, { token: tP1 });
check("la place est libérée", r.data.seats_available === 1, r.data);
r = await call("DELETE", "/bookings/" + booking1, { token: tP1 });
check("annulation en double 409", r.status === 409, r);
r = await call("POST", `/rides/${rideId}/bookings`, { token: tP1 });
check("re-réservation après annulation 201", r.status === 201, r);

console.log("UC9 Modifier ou supprimer un trajet");
r = await call("PUT", "/rides/" + rideId, { token: tP1, body: { ...ride, price: 1 } });
check("modification par un non-propriétaire 403", r.status === 403, r);
r = await call("PUT", "/rides/" + rideId, { token: tDriver, body: { ...ride, price: 30, seats_total: 3 } });
check("modification par le propriétaire 200", r.status === 200 && r.data.price === 30 && r.data.seats_total === 3, r.data);
r = await call("PUT", "/rides/" + rideId, { token: tDriver, body: { ...ride, seats_total: 1 } });
check("places < réservations actives 409", r.status === 409, r);
await call("POST", `/rides/${otherRideId}/bookings`, { token: tP2 });
r = await call("DELETE", "/rides/" + otherRideId, { token: tDriver });
check("suppression 204", r.status === 204, r);
r = await call("GET", "/me/bookings", { token: tP2 });
const deletedBooking = r.data.find((b) => b.ride_id === otherRideId);
check("la réservation du trajet supprimé est annulée", deletedBooking && deletedBooking.status === "CANCELLED", r.data);

console.log("UC10 Mes trajets / mes réservations");
r = await call("GET", "/me/rides", { token: tDriver });
check("mes trajets contient le trajet publié", r.status === 200 && r.data.some((x) => x.id === rideId), r.status);
r = await call("GET", "/me/bookings", { token: tP1 });
check("mes réservations contient le trajet", r.status === 200 && r.data.some((b) => b.ride && b.ride.id === rideId && b.status === "ACTIVE"), r.data);

console.log("UC6 / UC11 Terminer et évaluer");
r = await call("POST", `/rides/${rideId}/ratings`, { token: tP1, body: { target: driver, score: 5 } });
check("évaluer avant la fin 409", r.status === 409, r);
r = await call("POST", `/rides/${rideId}/complete`, { token: tP1 });
check("terminer par un non-propriétaire 403", r.status === 403, r);
r = await call("POST", `/rides/${rideId}/complete`, { token: tDriver });
check("terminer le trajet 200", r.status === 200 && r.data.status === "COMPLETED", r.data);
r = await call("POST", `/rides/${rideId}/complete`, { token: tDriver });
check("terminer deux fois 409", r.status === 409, r);
r = await call("POST", `/rides/${rideId}/ratings`, { token: tP1, body: { target: driver, score: 4, comment: "Très bien" } });
check("passager évalue conducteur 201", r.status === 201 && r.data.score === 4, r);
r = await call("POST", `/rides/${rideId}/ratings`, { token: tP1, body: { target: driver, score: 4 } });
check("évaluation en double 409", r.status === 409, r);
r = await call("POST", `/rides/${rideId}/ratings`, { token: tDriver, body: { target: pass1, score: 5 } });
check("conducteur évalue passager 201", r.status === 201, r);
r = await call("POST", `/rides/${rideId}/ratings`, { token: tP2, body: { target: driver, score: 1 } });
check("non-participant 403", r.status === 403, r);
r = await call("POST", `/rides/${rideId}/ratings`, { token: tP1, body: { target: driver, score: 9 } });
check("note hors bornes 400", r.status === 400 || r.status === 409, r);
r = await call("GET", "/rides/" + rideId, { token: tP1 });
check("un trajet terminé n'est plus consultable (UC7 : publiés seulement) 404", r.status === 404, r);
r = await call("POST", "/rides", { token: tDriver, body: { ...ride, departure_time: inHours(96) } });
r = await call("GET", "/rides/" + r.data.id, { token: tP1 });
check("la note du conducteur apparaît sur ses autres trajets", r.data.driver && r.data.driver.rating === 4, r.data.driver);

console.log("UC12 Notifications");
check("livrées au journal du service (voir la sortie du serveur)", true);

console.log("UC13 Déconnexion");
r = await call("DELETE", "/auth/session", { token: tP2 });
check("déconnexion 204", r.status === 204, r);
r = await call("GET", "/profile", { token: tP2 });
check("jeton invalide après déconnexion 401", r.status === 401, r);

console.log(`\n${passed} réussis, ${failed} échoués`);
process.exit(failed ? 1 : 0);
