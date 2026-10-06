package worker

import (
	"context"
	"log"
	"time"

	"indico-test-be/internal/config"
	"indico-test-be/internal/service"
)

type CleanupWorker struct {
	cfg              *config.Config
	inventoryService service.InventoryService
}

func NewCleanupWorker(cfg *config.Config, inventoryService service.InventoryService) *CleanupWorker {
	return &CleanupWorker{
		cfg:              cfg,
		inventoryService: inventoryService,
	}
}

func (w *CleanupWorker) Start(ctx context.Context) {
	interval := time.Duration(w.cfg.CleanupIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Cleanup worker started with interval: %v", interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping cleanup worker...")
			return
		case <-ticker.C:
			count, err := w.inventoryService.CleanupExpiredReservations(ctx)
			if err != nil {
				log.Printf("Failed to cleanup expired reservations: %v", err)
			} else if count > 0 {
				log.Printf("Successfully expired and released %d reservations", count)
			}
		}
	}
}
