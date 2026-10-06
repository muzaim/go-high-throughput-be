package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"indico-test-be/internal/config"
	"indico-test-be/internal/handler"
	"indico-test-be/internal/model"
	"indico-test-be/internal/repository"
	"indico-test-be/internal/routes"
	"indico-test-be/internal/service"
	ws "indico-test-be/internal/websocket"
	"indico-test-be/internal/worker"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	runDatabaseMigrations(cfg)

	db, err := repository.NewDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	itemRepo := repository.NewItemRepository(db.GetDB())
	reservationRepo := repository.NewReservationRepository(db.GetDB())

	hub := ws.NewHub()
	go hub.Run()

	inventoryService := service.NewInventoryService(cfg, db, itemRepo, reservationRepo, hub)

	hub.SetItemFetcher(func() ([]model.StockResponse, error) {
		return inventoryService.GetAllItems(context.Background())
	})

	inventoryHandler := handler.NewInventoryHandler(inventoryService)
	router := routes.SetupRouter(inventoryHandler, hub)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	cleanupWorker := worker.NewCleanupWorker(cfg, inventoryService)
	go cleanupWorker.Start(workerCtx)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.AppPort),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Server running on port %s in [%s] mode...", cfg.AppPort, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed to listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server gracefully...")

	workerCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced shutdown: %v", err)
	}

	log.Println("Server exited successfully.")
}

func runDatabaseMigrations(cfg *config.Config) {
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBSSLMode)

	m, err := migrate.New("file://migrations", dbURL)
	if err != nil {
		log.Printf("Could not initialize migrate: %v", err)
		return
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Printf("Database migration error: %v", err)
	} else {
		log.Println("Database migration executed successfully.")
	}
}
