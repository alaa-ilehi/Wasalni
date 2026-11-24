// Command wasalni démarre l'API Wasalni : racine de composition et serveur HTTP.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/api"
	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/photos"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/sqlite"
)

func main() {
	adresse := flag.String("addr", valeurEnv("WASALNI_ADDR", ":8080"), "adresse d'écoute HTTP")
	cheminBase := flag.String("db", valeurEnv("WASALNI_DB", "wasalni.db"), "fichier SQLite")
	flag.Parse()

	journal := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(journal)

	ctx, arret := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer arret()

	// Racine de composition : les adaptateurs sont choisis ici, une seule fois.
	db, err := sqlite.Ouvrir(ctx, *cheminBase)
	if err != nil {
		journal.Error("base de données indisponible", "erreur", err)
		os.Exit(1)
	}
	defer db.Close()

	profils := sqlite.NouveauProfilRepository(db)
	gerer := application.NouveauGererProfil(profils, normaliseurPhoto{})

	serveur := &http.Server{
		Addr:              *adresse,
		Handler:           api.Routes(gerer),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		journal.Info("écoute HTTP", "adresse", *adresse, "base", *cheminBase)
		if err := serveur.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			journal.Error("serveur arrêté", "erreur", err)
			arret()
		}
	}()

	<-ctx.Done()
	journal.Info("arrêt demandé, fermeture propre")
	delai, annuler := context.WithTimeout(context.Background(), 10*time.Second)
	defer annuler()
	if err := serveur.Shutdown(delai); err != nil {
		journal.Error("fermeture incomplète", "erreur", err)
	}
}

// normaliseurPhoto adapte la fonction du paquet photos au port application.NormaliseurPhoto.
type normaliseurPhoto struct{}

func (normaliseurPhoto) Normaliser(photoBase64 string) (string, error) {
	return photos.Normaliser(photoBase64)
}

func valeurEnv(cle, defaut string) string {
	if v := os.Getenv(cle); v != "" {
		return v
	}
	return defaut
}
