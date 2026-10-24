// Command server runs the Photo Backup Manager backend.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/Manukesharwani09/photo-backup-manager/internal/api"
	"github.com/Manukesharwani09/photo-backup-manager/internal/config"
	"github.com/Manukesharwani09/photo-backup-manager/internal/google"
	"github.com/Manukesharwani09/photo-backup-manager/internal/store"
)

func main() {
	envFile := flag.String("env", ".env", "path to the .env file")
	flag.Parse()

	cfg, err := config.Load(*envFile)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	if err := st.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	userID, err := st.DefaultUserID(ctx)
	if err != nil {
		log.Fatalf("default user: %v", err)
	}

	srv := &api.Server{
		Store:    st,
		OAuth:    google.OAuthConfig(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL),
		TokenKey: cfg.TokenKey,
		UserID:   userID,
	}
	log.Printf("listening on http://%s  (connect an account: http://%s/oauth/start?role=primary)", cfg.ListenAddr, cfg.ListenAddr)
	log.Fatal(http.ListenAndServe(cfg.ListenAddr, srv.Routes()))
}
