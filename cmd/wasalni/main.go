// Command wasalni démarre l'API Wasalni : racine de composition et serveur HTTP.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/api"
	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/notifications"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/passwords"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/photos"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/sqlite"
)

// notificateurComplet réunit les quatre ports de notification : un seul adaptateur, choisi une fois au démarrage.
type notificateurComplet interface {
	application.Notificateur
	application.NotificateurAnnulation
	application.NotificateurGestionTrajet
	application.NotificateurFinTrajet
}

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
	// UC12 : une URL configurée livre réellement les événements ; sans elle, le journal reste la trace (ADR 0013).
	var notificateur notificateurComplet = notifications.NouveauJournal(journal)
	if url := valeurEnv("WASALNI_NOTIFICATION_URL", ""); url != "" {
		notificateur = notifications.NouveauWebhook(url)
	}
	reserverPlace := application.NouveauReserverPlace(
		trajets,
		reservations,
		notificateur,
	)
	annulerReservation := application.NouveauAnnulerReservation(reservations, horlogeReelle{}, notificateur)
	evaluerParticipant := application.NouveauEvaluerParticipant(trajets, reservations, evaluations)
	terminerTrajet := application.NouveauTerminerTrajet(trajets, notificateur)
	gererTrajet := application.NouveauGererTrajet(trajets, trajets, horlogeReelle{}).AvecNotificateur(notificateur)

	// UC10 : deux listes en lecture seule, filtrées sur l'identité de la session.
	mesTrajets := application.NouveauMesTrajets(trajets)
	mesReservations := application.NouveauMesReservations(reservations, trajets)

	// UC13 : les comptes et les sessions vivent à côté des profils ; aucun jeton n'est stocké en clair.
	authentification := application.NouvelleAuthentification(
		sqlite.NouveauCredentialRepository(db),
		sqlite.NouveauSessionRepository(db),
		passwords.Hacheur{},
		horlogeReelle{},
	)

	serveur := &http.Server{
		Addr: *adresse,
		Handler: api.Routes(api.Deps{
			Profils:          gererProfil,
			Recherches:       rechercherTrajets,
			Trajets:          publierTrajet,
			Details:          consulterTrajet,
			Reservations:     reserverPlace,
			Annulations:      annulerReservation,
			Terminaisons:     terminerTrajet,
			GestionTrajets:   gererTrajet,
			Evaluations:      evaluerParticipant,
			MesTrajets:       mesTrajets,
			MesReservations:  mesReservations,
			Authentification: authentification,
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
	normalisee, err := photos.Normaliser(photoBase64)
	// Le budget de pixels (ADR 0008) remonte sous une erreur d'application, sans que la couche applicative connaisse
	// le paquet photos.
	if errors.Is(err, photos.ErrImageTropGrande) {
		return "", fmt.Errorf("%w : %v", application.ErrImageTropGrande, err)
	}
	return normalisee, err
}

func valeurEnv(cle, defaut string) string {
	if v := os.Getenv(cle); v != "" {
		return v
	}
	return defaut
}
