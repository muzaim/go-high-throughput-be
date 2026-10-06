package model

import "time"

type Item struct {
	ID            string    `json:"item_id"`
	Name          string    `json:"name"`
	TotalStock    int       `json:"total_stock"`
	ReservedStock int       `json:"reserved_stock"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (i *Item) AvailableStock() int {
	available := i.TotalStock - i.ReservedStock
	if available < 0 {
		return 0
	}
	return available
}

type StockResponse struct {
	ItemID         string `json:"item_id"`
	Name           string `json:"name"`
	TotalStock     int    `json:"total_stock"`
	ReservedStock  int    `json:"reserved_stock"`
	AvailableStock int    `json:"available_stock"`
}

type ItemListResponse struct {
	Status string          `json:"status"`
	Items  []StockResponse `json:"items"`
}

type ItemDetailResponse struct {
	Status string        `json:"status"`
	Item   StockResponse `json:"item"`
}
