package api_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/api"
	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/photos"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/sqlite"
)

type banc struct {
	serveur *httptest.Server
}

func nouveauBanc(t *testing.T) *banc {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Ouvrir(ctx, filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	t.Cleanup(func() { db.Close() })

	gerer := application.NouveauGererProfil(sqlite.NouveauProfilRepository(db), adaptateurPhoto{})
	serveur := httptest.NewServer(api.Routes(gerer))
	t.Cleanup(serveur.Close)
	return &banc{serveur: serveur}
}

// adaptateurPhoto relie le paquet photos au port du cas d'usage.
type adaptateurPhoto struct{}

func (adaptateurPhoto) Normaliser(photo string) (string, error) { return photos.Normaliser(photo) }

func (b *banc) appel(t *testing.T, methode, chemin, acteur, corps string) (int, map[string]any) {
	t.Helper()
	var lecteur io.Reader
	if corps != "" {
		lecteur = strings.NewReader(corps)
	}
	requete, err := http.NewRequest(methode, b.serveur.URL+chemin, lecteur)
	if err != nil {
		t.Fatalf("requête : %v", err)
	}
	if acteur != "" {
		requete.Header.Set(api.EnTeteIdentite, acteur)
	}
	reponse, err := http.DefaultClient.Do(requete)
	if err != nil {
		t.Fatalf("appel %s %s : %v", methode, chemin, err)
	}
	defer reponse.Body.Close()
	brut, _ := io.ReadAll(reponse.Body)
	var charge map[string]any
	if len(brut) > 0 {
		if err := json.Unmarshal(brut, &charge); err != nil {
			t.Fatalf("réponse non JSON (%d) : %s", reponse.StatusCode, brut)
		}
	}
	return reponse.StatusCode, charge
}

func corpsProfil(nom string, age int, sexe string, photo *string) string {
	charge := map[string]any{"name": nom, "age": age}
	if sexe != "" {
		charge["sex"] = sexe
	}
	if photo != nil {
		charge["photo"] = *photo
	}
	brut, _ := json.Marshal(charge)
	return string(brut)
}

func TestIdentiteObligatoire(t *testing.T) {
	b := nouveauBanc(t)
	for _, cas := range []struct{ methode, corps string }{
		{http.MethodGet, ""},
		{http.MethodPut, corpsProfil("Alice", 27, "", nil)},
	} {
		statut, charge := b.appel(t, cas.methode, "/profile", "", cas.corps)
		if statut != http.StatusUnauthorized {
			t.Errorf("%s sans X-User : statut = %d, attendu 401", cas.methode, statut)
		}
		if charge["error"] == nil {
			t.Errorf("%s sans X-User : corps d'erreur absent", cas.methode)
		}
	}
}

func TestValidationDuProfil(t *testing.T) {
	b := nouveauBanc(t)
	for _, corps := range []string{
		`{"age": 27}`,
		`{"name": "  ", "age": 27}`,
		`{"name": "A", "age": 27}`,
		`{"name": "Alice", "age": 12}`,
		`{"name": "Alice", "age": 120}`,
		`{"name": "Alice", "age": 27, "sex": "HOMME"}`,
		`{"name": "Alice", "age": 27, "champ_inconnu": 1}`,
		`{"name": "Alice", "age": "vingt-sept"}`,
	} {
		statut, charge := b.appel(t, http.MethodPut, "/profile", "alice", corps)
		if statut != http.StatusBadRequest {
			t.Errorf("corps %s : statut = %d, attendu 400", corps, statut)
		}
		if charge["error"] == nil {
			t.Errorf("corps %s : message d'erreur absent", corps)
		}
	}
}

func TestCreationPuisMiseAJour(t *testing.T) {
	b := nouveauBanc(t)

	statut, cree := b.appel(t, http.MethodPut, "/profile", "alice", corpsProfil("Alice Ben Salah", 27, "FEMALE", nil))
	if statut != http.StatusCreated {
		t.Fatalf("création : statut = %d, attendu 201 (corps %v)", statut, cree)
	}
	if cree["id"] != "alice" || cree["name"] != "Alice Ben Salah" || cree["sex"] != "FEMALE" {
		t.Errorf("profil créé = %v", cree)
	}
	if cree["created_at"] == "" || cree["updated_at"] == "" {
		t.Errorf("horodatages absents : %v", cree)
	}

	statut, maj := b.appel(t, http.MethodPut, "/profile", "alice", corpsProfil("Alice B.", 28, "", nil))
	if statut != http.StatusOK {
		t.Fatalf("mise à jour : statut = %d, attendu 200 (corps %v)", statut, maj)
	}
	if maj["name"] != "Alice B." || maj["age"] != float64(28) {
		t.Errorf("profil mis à jour = %v", maj)
	}

	statut, lu := b.appel(t, http.MethodGet, "/profile", "alice", "")
	if statut != http.StatusOK {
		t.Fatalf("lecture : statut = %d, attendu 200", statut)
	}
	if lu["name"] != "Alice B." || lu["id"] != "alice" {
		t.Errorf("profil relu = %v", lu)
	}
}

func TestProfilInconnu(t *testing.T) {
	b := nouveauBanc(t)
	statut, charge := b.appel(t, http.MethodGet, "/profile", "personne", "")
	if statut != http.StatusNotFound {
		t.Errorf("statut = %d, attendu 404", statut)
	}
	if charge["error"] == nil {
		t.Error("message d'erreur absent")
	}
}

func TestIsolationEntreIdentites(t *testing.T) {
	b := nouveauBanc(t)
	b.appel(t, http.MethodPut, "/profile", "alice", corpsProfil("Alice", 27, "", nil))
	b.appel(t, http.MethodPut, "/profile", "bob", corpsProfil("Bob", 31, "MALE", nil))

	_, lu := b.appel(t, http.MethodGet, "/profile", "bob", "")
	if lu["name"] != "Bob" {
		t.Errorf("le profil de bob a été modifié : %v", lu)
	}
	_, luAlice := b.appel(t, http.MethodGet, "/profile", "alice", "")
	if luAlice["name"] != "Alice" {
		t.Errorf("le profil d'alice a été modifié : %v", luAlice)
	}
}

// grandePhoto renvoie une image PNG lourde pour vérifier la normalisation de bout en bout.
func grandePhoto(t *testing.T) string {
	t.Helper()
	largeur, hauteur := 900, 700
	source := image.NewRGBA(image.Rect(0, 0, largeur, hauteur))
	for y := 0; y < hauteur; y++ {
		for x := 0; x < largeur; x++ {
			source.SetRGBA(x, y, color.RGBA{
				R: uint8((x * 7) % 256),
				G: uint8((y * 5) % 256),
				B: uint8((x*y + 13) % 256),
				A: 0xff,
			})
		}
	}
	var tampon bytes.Buffer
	if err := png.Encode(&tampon, source); err != nil {
		t.Fatalf("image de test : %v", err)
	}
	return base64.StdEncoding.EncodeToString(tampon.Bytes())
}

func TestPhotoNormaliseeALEnregistrement(t *testing.T) {
	b := nouveauBanc(t)
	statut, profil := b.appel(t, http.MethodPut, "/profile", "alice",
		corpsProfil("Alice", 27, "", pointer(grandePhoto(t))))
	if statut != http.StatusCreated {
		t.Fatalf("statut = %d, attendu 201 (corps %v)", statut, profil)
	}
	photo, _ := profil["photo"].(string)
	if photo == "" {
		t.Fatal("la photo normalisée doit être renvoyée")
	}
	brut, err := base64.StdEncoding.DecodeString(photo)
	if err != nil {
		t.Fatalf("photo non base64 : %v", err)
	}
	if len(brut) > photos.PoidsMax {
		t.Errorf("photo stockée = %d octets, attendu au plus %d", len(brut), photos.PoidsMax)
	}
	img, format, err := image.Decode(bytes.NewReader(brut))
	if err != nil || format != "png" {
		t.Fatalf("photo stockée = %q (erreur %v), attendu un PNG", format, err)
	}
	if b := img.Bounds(); b.Dx() > photos.CoteMax || b.Dy() > photos.CoteMax {
		t.Errorf("dimensions = %dx%d, attendu au plus %d", b.Dx(), b.Dy(), photos.CoteMax)
	}

	// une mise à jour sans photo la conserve
	_, maj := b.appel(t, http.MethodPut, "/profile", "alice", corpsProfil("Alice", 28, "", nil))
	if maj["photo"] != photo {
		t.Error("la photo doit être conservée quand le champ est absent")
	}

	// une photo vide l'efface
	_, efface := b.appel(t, http.MethodPut, "/profile", "alice", corpsProfil("Alice", 28, "", pointer("")))
	if _, present := efface["photo"]; present {
		t.Errorf("la photo devait être effacée : %v", efface)
	}
}

func TestPhotoIllisible(t *testing.T) {
	b := nouveauBanc(t)
	statut, charge := b.appel(t, http.MethodPut, "/profile", "alice",
		corpsProfil("Alice", 27, "", pointer("pas-du-tout-une-image")))
	if statut != http.StatusBadRequest {
		t.Errorf("statut = %d, attendu 400", statut)
	}
	if charge["error"] == nil {
		t.Error("message d'erreur absent")
	}
}

func TestCorpsTropVolumineux(t *testing.T) {
	b := nouveauBanc(t)
	enorme := strings.Repeat("A", photos.TailleMaxCorps+1024)
	statut, _ := b.appel(t, http.MethodPut, "/profile", "alice",
		corpsProfil("Alice", 27, "", pointer(enorme)))
	if statut != http.StatusRequestEntityTooLarge {
		t.Errorf("statut = %d, attendu 413", statut)
	}
}

func pointer(valeur string) *string { return &valeur }
