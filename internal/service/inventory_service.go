package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"indico-test-be/internal/config"
	"indico-test-be/internal/model"
	"indico-test-be/internal/repository"
	ws "indico-test-be/internal/websocket"

	"github.com/google/uuid"
)

type InventoryService interface {
	ReserveStock(ctx context.Context, req *model.ReserveRequest) (*model.ReserveResponse, error)
	ConfirmReservation(ctx context.Context, req *model.ConfirmRequest) (*model.ConfirmResponse, error)
	GetStock(ctx context.Context, itemID string) (*model.StockResponse, error)
	GetAllItems(ctx context.Context) ([]model.StockResponse, error)
	GetItemDetail(ctx context.Context, itemID string) (*model.StockResponse, error)
	CleanupExpiredReservations(ctx context.Context) (int64, error)
}

type inventoryService struct {
	cfg             *config.Config
	transactor      repository.Transactor
	itemRepo        repository.ItemRepository
	reservationRepo repository.ReservationRepository
	hub             *ws.Hub
}

func NewInventoryService(
	cfg *config.Config,
	transactor repository.Transactor,
	itemRepo repository.ItemRepository,
	reservationRepo repository.ReservationRepository,
	hub *ws.Hub,
) InventoryService {
	return &inventoryService{
		cfg:             cfg,
		transactor:      transactor,
		itemRepo:        itemRepo,
		reservationRepo: reservationRepo,
		hub:             hub,
	}
}

func (s *inventoryService) broadcastItemStock(ctx context.Context, itemID string) {
	if s.hub == nil {
		return
	}
	stock, err := s.GetStock(ctx, itemID)
	if err == nil && stock != nil {
		s.hub.BroadcastStock(*stock)
	}
}

func (s *inventoryService) ReserveStock(ctx context.Context, req *model.ReserveRequest) (*model.ReserveResponse, error) {
	if req.Quantity <= 0 {
		return nil, model.ErrInvalidQuantity
	}

	var resResponse *model.ReserveResponse

	err := s.transactor.ExecTx(ctx, func(tx *sql.Tx) error {
		item, err := s.itemRepo.FindByIDWithLock(ctx, tx, req.ItemID)
		if err != nil {
			return err
		}

		availableStock := item.AvailableStock()
		if availableStock < req.Quantity {
			return model.ErrInsufficientStock
		}

		resID := fmt.Sprintf("res_%s", uuid.New().String())
		now := time.Now().UTC()
		expiresAt := now.Add(time.Duration(s.cfg.ReservationExpirationMinutes) * time.Minute)

		reservation := &model.Reservation{
			ID:        resID,
			UserID:    req.UserID,
			ItemID:    req.ItemID,
			Quantity:  req.Quantity,
			Status:    model.StatusActive,
			ExpiresAt: expiresAt,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := s.itemRepo.IncrementReservedStock(ctx, tx, req.ItemID, req.Quantity); err != nil {
			return err
		}

		if err := s.reservationRepo.Create(ctx, tx, reservation); err != nil {
			return err
		}

		resResponse = &model.ReserveResponse{
			Status:        "success",
			ReservationID: resID,
			ItemID:        req.ItemID,
			Quantity:      req.Quantity,
			ExpiresAt:     expiresAt,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	s.broadcastItemStock(ctx, req.ItemID)

	return resResponse, nil
}

func (s *inventoryService) ConfirmReservation(ctx context.Context, req *model.ConfirmRequest) (*model.ConfirmResponse, error) {
	var confirmResp *model.ConfirmResponse
	var itemID string

	err := s.transactor.ExecTx(ctx, func(tx *sql.Tx) error {
		res, err := s.reservationRepo.FindByIDWithLock(ctx, tx, req.ReservationID)
		if err != nil {
			return err
		}

		itemID = res.ItemID

		if res.Status == model.StatusConfirmed {
			return model.ErrReservationAlreadyConfirmed
		}

		now := time.Now().UTC()

		if res.Status == model.StatusExpired || now.After(res.ExpiresAt) {
			if res.Status == model.StatusActive {
				_ = s.reservationRepo.UpdateStatus(ctx, tx, res.ID, model.StatusExpired, nil)
				_ = s.itemRepo.ReleaseReservedStock(ctx, tx, res.ItemID, res.Quantity)
			}
			return model.ErrReservationExpired
		}

		_, err = s.itemRepo.FindByIDWithLock(ctx, tx, res.ItemID)
		if err != nil {
			return err
		}

		if err := s.reservationRepo.UpdateStatus(ctx, tx, res.ID, model.StatusConfirmed, &now); err != nil {
			return err
		}

		if err := s.itemRepo.CommitPhysicalStock(ctx, tx, res.ItemID, res.Quantity); err != nil {
			return err
		}

		confirmResp = &model.ConfirmResponse{
			Status:        "success",
			ReservationID: res.ID,
			ConfirmedAt:   now,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if itemID != "" {
		s.broadcastItemStock(ctx, itemID)
	}

	return confirmResp, nil
}

func (s *inventoryService) GetStock(ctx context.Context, itemID string) (*model.StockResponse, error) {
	if itemID == "" {
		return nil, model.ErrItemNotFound
	}

	item, err := s.itemRepo.FindByID(ctx, nil, itemID)
	if err != nil {
		return nil, err
	}

	return &model.StockResponse{
		ItemID:         item.ID,
		Name:           item.Name,
		TotalStock:     item.TotalStock,
		ReservedStock:  item.ReservedStock,
		AvailableStock: item.AvailableStock(),
	}, nil
}

func (s *inventoryService) GetAllItems(ctx context.Context) ([]model.StockResponse, error) {
	items, err := s.itemRepo.FindAll(ctx, nil)
	if err != nil {
		return nil, err
	}

	responses := make([]model.StockResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, model.StockResponse{
			ItemID:         item.ID,
			Name:           item.Name,
			TotalStock:     item.TotalStock,
			ReservedStock:  item.ReservedStock,
			AvailableStock: item.AvailableStock(),
		})
	}

	return responses, nil
}

func (s *inventoryService) GetItemDetail(ctx context.Context, itemID string) (*model.StockResponse, error) {
	if itemID == "" {
		return nil, model.ErrItemNotFound
	}

	item, err := s.itemRepo.FindByID(ctx, nil, itemID)
	if err != nil {
		return nil, err
	}

	return &model.StockResponse{
		ItemID:         item.ID,
		Name:           item.Name,
		TotalStock:     item.TotalStock,
		ReservedStock:  item.ReservedStock,
		AvailableStock: item.AvailableStock(),
	}, nil
}

func (s *inventoryService) CleanupExpiredReservations(ctx context.Context) (int64, error) {
	var count int64
	err := s.transactor.ExecTx(ctx, func(tx *sql.Tx) error {
		var err error
		count, err = s.reservationRepo.ExpireBatch(ctx, tx, 100)
		return err
	})

	if count > 0 {
		items, err := s.GetAllItems(ctx)
		if err == nil && s.hub != nil {
			for _, item := range items {
				s.hub.BroadcastStock(item)
			}
		}
	}

	return count, err
}
