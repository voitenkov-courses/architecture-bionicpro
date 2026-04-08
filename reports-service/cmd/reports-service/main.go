package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/voitenkov-courses/architecture-bionicpro/reports-service/internal/config"
	internalhttp "github.com/voitenkov-courses/architecture-bionicpro/reports-service/internal/controllers/httpserver"
	"github.com/voitenkov-courses/architecture-bionicpro/reports-service/internal/dal/cdn"
	"github.com/voitenkov-courses/architecture-bionicpro/reports-service/internal/dal/storage"
	"github.com/voitenkov-courses/architecture-bionicpro/reports-service/internal/services/logger"
	"github.com/voitenkov-courses/architecture-bionicpro/reports-service/internal/services/reports"
)

const host = "localhost"

var (
	configFile string
	wg         *sync.WaitGroup
)

func init() {
	flag.StringVar(&configFile, "config", "/etc/reports-service/config.yaml", "Path to configuration file")
}

func main() {
	flag.Parse()

	cfg, err := config.Parse(configFile)
	if err != nil {
		log.Fatal(err)
	}

	storage := storage.New(cfg)
	cdn := cdn.New(cfg)
	reportsService := reports.New(storage, cdn)
	logg := logger.New(cfg.Logger.Level)
	ctx := context.Background()

	cdn.InitClientAndBucket(ctx)
	storage.Connect(ctx)
	defer storage.Close()

	server := internalhttp.NewServer(cfg, reportsService, logg)

	ctx, cancel := signal.NotifyContext(ctx,
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()

	go func() {
		<-ctx.Done()
		logg.Info("Shutting down auth server...")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()

		if err := server.Stop(ctx); err != nil {
			logg.Error("Failed to stop reports server: " + err.Error())
		}
	}()

	logg.Info("Auth server is running...")
	wg = &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := server.Start(ctx, host); err != nil {
			logg.Error("Failed to start reports server: " + err.Error())
		}
	}()

	wg.Wait()
	logg.Info("Reports server exited properly")
	cancel()
	os.Exit(1) //nolint:gocritic
}
