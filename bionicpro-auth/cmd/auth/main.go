package main

import (
	"context"
	"flag"
	"log"
	"math/rand/v2"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/voitenkov-courses/architecture-bionicpro/bionicpro-auth/internal/config"
	internalhttp "github.com/voitenkov-courses/architecture-bionicpro/bionicpro-auth/internal/controllers/httpserver"
	"github.com/voitenkov-courses/architecture-bionicpro/bionicpro-auth/internal/services/auth"
	"github.com/voitenkov-courses/architecture-bionicpro/bionicpro-auth/internal/services/logger"
	"github.com/voitenkov-courses/architecture-bionicpro/bionicpro-auth/internal/services/session"
)

const host = "localhost"

var (
	configFile   string
	randomSource *rand.Rand
	wg           *sync.WaitGroup
)

func init() {
	flag.StringVar(&configFile, "config", "../../configs/config.yaml", "Path to configuration file")
	randomSource = rand.New(rand.NewPCG(1, 2))
}

func main() {
	flag.Parse()

	cfg, err := config.Parse(configFile)
	if err != nil {
		log.Fatal(err)
	}

	logg := logger.New(cfg.Logger.Level)
	authContext := auth.NewAuthContext(cfg)

	authContext.InitOIDC(host)

	sessionStore := session.New(cfg.StateTTLSeconds, cfg.SessionTTLSeconds)
	server := internalhttp.NewServer(cfg, authContext, logg, randomSource, sessionStore)

	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()

	go func() {
		<-ctx.Done()
		logg.Info("Shutting down auth server...")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()

		if err := server.Stop(ctx); err != nil {
			logg.Error("Failed to stop auth server: " + err.Error())
		}
	}()

	logg.Info("Auth server is running...")
	wg = &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := server.Start(ctx, host); err != nil {
			logg.Error("Failed to start auth server: " + err.Error())
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := server.ScheduledCleanup(ctx); err != nil {
			logg.Error("Failed to start sessions cleanup scheduler: " + err.Error())
		}
	}()

	wg.Wait()
	logg.Info("Auth server exited properly")
	cancel()
	os.Exit(1) //nolint:gocritic
}
