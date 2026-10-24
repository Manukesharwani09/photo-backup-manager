// Command main runs the Photo Backup Manager backend.
//
//	go run ./cmd appserver   # migrate, start the cloud-check scheduler, serve HTTP
//	go run ./cmd migrate     # apply database migrations and exit
//	go run ./cmd rollback 1  # revert the newest N migrations
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/Manukesharwani09/photo-backup-manager/internal/api"
	"github.com/Manukesharwani09/photo-backup-manager/internal/config"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/client/google"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"
)

const usage = `usage: main [-env FILE] <command>

commands:
  appserver     apply migrations, then serve the HTTP API
  migrate       apply migrations and exit
  rollback N    revert the newest N applied migrations`

func main() {
	envFile := flag.String("env", ".env", "path to the .env file")
	flag.Usage = func() { fmt.Fprintln(os.Stderr, usage) }
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}

	cfg, err := config.Load(*envFile)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx := context.Background()
	st, err := repository.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	switch flag.Arg(0) {
	case "rollback":
		steps, err := strconv.Atoi(flag.Arg(1))
		if err != nil || steps < 1 {
			log.Fatal("usage: rollback N (N >= 1)")
		}
		if err := st.Rollback(ctx, steps); err != nil {
			log.Fatalf("rollback: %v", err)
		}
		log.Printf("rolled back %d migration(s)", steps)
		return
	case "migrate", "appserver":
	default:
		flag.Usage()
		os.Exit(2)
	}

	if err := st.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	switch flag.Arg(0) {
	case "migrate":
		log.Print("migrations applied")
	case "appserver":
		runAppServer(ctx, cfg, st)
	default:
		flag.Usage()
		os.Exit(2)
	}
}

func runAppServer(ctx context.Context, cfg config.Config, st *repository.Store) {
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
	go srv.RunCloudChecks(ctx, cfg.CloudCheckInterval)
	log.Printf("listening on http://%s  (connect an account: http://%s/oauth/start?role=primary)", cfg.ListenAddr, cfg.ListenAddr)
	log.Fatal(http.ListenAndServe(cfg.ListenAddr, srv.Routes()))
}
