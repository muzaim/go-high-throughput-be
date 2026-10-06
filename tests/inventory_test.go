package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"indico-test-be/internal/config"
	"indico-test-be/internal/handler"
	"indico-test-be/internal/model"
	"indico-test-be/internal/repository"
	"indico-test-be/internal/routes"
	"indico-test-be/internal/service"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*sql.DB, repository.Transactor, repository.ItemRepository, repository.ReservationRepository, service.InventoryService, *gin.Engine) {
	gin.SetMode(gin.TestMode)

	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}
	dbPass := os.Getenv("DB_PASSWORD")
	if dbPass == "" {
		dbPass = "postgres"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "flash_sale"
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		dbHost, dbUser, dbPass, dbName, dbPort)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("Skipping integration test: PostgreSQL unavailable (%v)", err)
		return nil, nil, nil, nil, nil, nil
	}

	if err := db.Ping(); err != nil {
		t.Skipf("Skipping integration test: PostgreSQL ping failed (%v)", err)
		return nil, nil, nil, nil, nil, nil
	}

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(25)

	_, _ = db.Exec("TRUNCATE TABLE reservations, items CASCADE;")

	cfg := &config.Config{
		AppPort:                      "8080",
		Env:                          "test",
		ReservationExpirationMinutes: 5,
		CleanupIntervalSeconds:       10,
	}

	transactor := repository.NewDBFromSQLDB(db)
	itemRepo := repository.NewItemRepository(db)
	resRepo := repository.NewReservationRepository(db)
	svc := service.NewInventoryService(cfg, transactor, itemRepo, resRepo)
	h := handler.NewInventoryHandler(svc)
	router := routes.SetupRouter(h)

	return db, transactor, itemRepo, resRepo, svc, router
}

func TestReserveStock_Success(t *testing.T) {
	db, _, itemRepo, _, _, router := setupTestDB(t)
	if db == nil {
		return
	}

	err := itemRepo.IncrementReservedStock(context.Background(), nil, "item_test_1", 0)
	require.Error(t, err)

	_, err = db.Exec("INSERT INTO items (id, name, total_stock, reserved_stock) VALUES ($1, $2, $3, $4)", "item_test_1", "Phone", 10, 0)
	require.NoError(t, err)

	reqBody := model.ReserveRequest{
		UserID:   "usr_100",
		ItemID:   "item_test_1",
		Quantity: 2,
	}
	jsonBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/inventory/reserve", bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.ReserveResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, "item_test_1", resp.ItemID)
	assert.Equal(t, 2, resp.Quantity)
	assert.NotEmpty(t, resp.ReservationID)

	var reservedStock int
	err = db.QueryRow("SELECT reserved_stock FROM items WHERE id = $1", "item_test_1").Scan(&reservedStock)
	assert.NoError(t, err)
	assert.Equal(t, 2, reservedStock)
}

func TestReserveStock_InsufficientStock(t *testing.T) {
	db, _, _, _, _, router := setupTestDB(t)
	if db == nil {
		return
	}

	_, err := db.Exec("INSERT INTO items (id, name, total_stock, reserved_stock) VALUES ($1, $2, $3, $4)", "item_test_2", "TV", 1, 0)
	require.NoError(t, err)

	reqBody := model.ReserveRequest{
		UserID:   "usr_101",
		ItemID:   "item_test_2",
		Quantity: 5,
	}
	jsonBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/inventory/reserve", bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestReserveStock_InvalidQuantity(t *testing.T) {
	_, _, _, _, _, router := setupTestDB(t)

	reqBody := map[string]interface{}{
		"user_id":  "usr_102",
		"item_id":  "item_test_3",
		"quantity": 0,
	}
	jsonBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/inventory/reserve", bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReserveStock_ItemNotFound(t *testing.T) {
	db, _, _, _, _, router := setupTestDB(t)
	if db == nil {
		return
	}

	reqBody := model.ReserveRequest{
		UserID:   "usr_103",
		ItemID:   "item_non_existent",
		Quantity: 1,
	}
	jsonBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/inventory/reserve", bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestConfirmReservation_Success(t *testing.T) {
	db, _, _, _, _, router := setupTestDB(t)
	if db == nil {
		return
	}

	_, err := db.Exec("INSERT INTO items (id, name, total_stock, reserved_stock) VALUES ($1, $2, $3, $4)", "item_test_4", "Laptop", 10, 2)
	require.NoError(t, err)

	_, err = db.Exec(
		"INSERT INTO reservations (id, user_id, item_id, quantity, status, expires_at) VALUES ($1, $2, $3, $4, $5, $6)",
		"res_test_confirm_1", "usr_104", "item_test_4", 2, "ACTIVE", time.Now().UTC().Add(5*time.Minute),
	)
	require.NoError(t, err)

	confirmReq := model.ConfirmRequest{ReservationID: "res_test_confirm_1"}
	jsonBytes, _ := json.Marshal(confirmReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/inventory/confirm", bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var totalStock, reservedStock int
	err = db.QueryRow("SELECT total_stock, reserved_stock FROM items WHERE id = $1", "item_test_4").Scan(&totalStock, &reservedStock)
	assert.NoError(t, err)
	assert.Equal(t, 8, totalStock)
	assert.Equal(t, 0, reservedStock)

	var status string
	err = db.QueryRow("SELECT status FROM reservations WHERE id = $1", "res_test_confirm_1").Scan(&status)
	assert.NoError(t, err)
	assert.Equal(t, "CONFIRMED", status)
}

func TestConfirmReservation_Expired(t *testing.T) {
	db, _, _, _, _, router := setupTestDB(t)
	if db == nil {
		return
	}

	_, err := db.Exec("INSERT INTO items (id, name, total_stock, reserved_stock) VALUES ($1, $2, $3, $4)", "item_test_5", "Watch", 10, 2)
	require.NoError(t, err)

	_, err = db.Exec(
		"INSERT INTO reservations (id, user_id, item_id, quantity, status, expires_at) VALUES ($1, $2, $3, $4, $5, $6)",
		"res_test_expired_1", "usr_105", "item_test_5", 2, "ACTIVE", time.Now().UTC().Add(-10*time.Minute),
	)
	require.NoError(t, err)

	confirmReq := model.ConfirmRequest{ReservationID: "res_test_expired_1"}
	jsonBytes, _ := json.Marshal(confirmReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/inventory/confirm", bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)

	var totalStock, reservedStock int
	err = db.QueryRow("SELECT total_stock, reserved_stock FROM items WHERE id = $1", "item_test_5").Scan(&totalStock, &reservedStock)
	assert.NoError(t, err)
	assert.Equal(t, 10, totalStock)
	assert.Equal(t, 0, reservedStock)
}

func TestReserveStock_HighConcurrency(t *testing.T) {
	db, _, _, _, _, router := setupTestDB(t)
	if db == nil {
		return
	}

	initialStock := 10
	concurrentRequests := 100
	itemID := "item_flash_sale_99"

	_, err := db.Exec("INSERT INTO items (id, name, total_stock, reserved_stock) VALUES ($1, $2, $3, $4)", itemID, "Flash Sale Console", initialStock, 0)
	require.NoError(t, err)

	var successCount int64
	var failureCount int64

	var wg sync.WaitGroup
	wg.Add(concurrentRequests)

	startSignal := make(chan struct{})

	for i := 0; i < concurrentRequests; i++ {
		userID := fmt.Sprintf("usr_concurrent_%d", i)
		go func(uid string) {
			defer wg.Done()

			<-startSignal

			reqBody := model.ReserveRequest{
				UserID:   uid,
				ItemID:   itemID,
				Quantity: 1,
			}
			jsonBytes, _ := json.Marshal(reqBody)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/inventory/reserve", bytes.NewBuffer(jsonBytes))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&failureCount, 1)
			}
		}(userID)
	}

	close(startSignal)
	wg.Wait()

	t.Logf("Concurrency Test Complete: Successes = %d, Failures = %d", successCount, failureCount)

	assert.Equal(t, int64(initialStock), successCount)
	assert.Equal(t, int64(concurrentRequests-initialStock), failureCount)

	var totalStock, reservedStock int
	err = db.QueryRow("SELECT total_stock, reserved_stock FROM items WHERE id = $1", itemID).Scan(&totalStock, &reservedStock)
	assert.NoError(t, err)

	assert.Equal(t, initialStock, totalStock)
	assert.Equal(t, initialStock, reservedStock)

	var activeResCount int64
	err = db.QueryRow("SELECT COUNT(*) FROM reservations WHERE item_id = $1 AND status = 'ACTIVE'", itemID).Scan(&activeResCount)
	assert.NoError(t, err)
	assert.Equal(t, int64(initialStock), activeResCount)
}
