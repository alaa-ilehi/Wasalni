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
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/notifications"
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
	gererProfil := application.NouveauGererProfil(profils, normaliseurPhoto{})

	// Un seul entrepôt de trajets sert la publication, recherche, lecture, détails et terminaison des trajets.
	trajets := sqlite.NouveauTrajetRepository(db)
	reservations := sqlite.NouveauReservationRepository(db)
	evaluations := sqlite.NouveauEvaluationRepository(db)
	publierTrajet := application.NouveauPublierTrajet(trajets, horlogeReelle{})
	rechercherTrajets := application.NouveauRechercherTrajets(trajets)
	consulterTrajet := application.NouveauConsulterTrajet(trajets, profils, evaluations)
	notificateur := notifications.NouveauJournal(journal)
	reserverPlace := application.NouveauReserverPlace(
		trajets,
		reservations,
		notificateur,
	)
	annulerReservation := application.NouveauAnnulerReservation(reservations, horlogeReelle{}, notificateur)
	evaluerParticipant := application.NouveauEvaluerParticipant(trajets, reservations, evaluations)
	terminerTrajet := application.NouveauTerminerTrajet(trajets, notifications.NouveauJournal(journal))
	gererTrajet := application.NouveauGererTrajet(trajets, trajets, horlogeReelle{}).AvecNotificateur(notifications.NouveauJournal(journal))

	serveur := &http.Server{
		Addr: *adresse,
		Handler: api.Routes(api.Deps{
			Profils:        gererProfil,
			Recherches:     rechercherTrajets,
			Trajets:        publierTrajet,
			Details:        consulterTrajet,
			Reservations:   reserverPlace,
			Annulations:    annulerReservation,
			Terminaisons:   terminerTrajet,
			GestionTrajets: gererTrajet,
			Evaluations:    evaluerParticipant,
		}),
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

// horlogeReelle adapte l'heure système au port application.Horloge.
type horlogeReelle struct{}

func (horlogeReelle) Maintenant() time.Time { return time.Now().UTC() }

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
