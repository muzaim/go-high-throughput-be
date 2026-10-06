package websocket

import (
	"log"
	"net/http"
	"sync"

	"indico-test-be/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WSMessage struct {
	Event string              `json:"event"`
	Data  model.StockResponse `json:"data"`
}

type ItemFetcherFunc func() ([]model.StockResponse, error)

type Hub struct {
	clients     map[*websocket.Conn]bool
	broadcast   chan model.StockResponse
	mu          sync.Mutex
	upgrader    websocket.Upgrader
	itemFetcher ItemFetcherFunc
}

func NewHub() *Hub {
	return &Hub{
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan model.StockResponse, 100),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (h *Hub) SetItemFetcher(fetcher ItemFetcherFunc) {
	h.itemFetcher = fetcher
}

func (h *Hub) Run() {
	for stock := range h.broadcast {
		msg := WSMessage{
			Event: "STOCK_UPDATED",
			Data:  stock,
		}

		h.mu.Lock()
		for client := range h.clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("WebSocket send error: %v", err)
				client.Close()
				delete(h.clients, client)
			}
		}
		h.mu.Unlock()
	}
}

func (h *Hub) HandleWS(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()

	if h.itemFetcher != nil {
		items, err := h.itemFetcher()
		if err == nil {
			for _, item := range items {
				_ = conn.WriteJSON(WSMessage{
					Event: "STOCK_UPDATED",
					Data:  item,
				})
			}
		}
	}

	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		h.mu.Unlock()
		conn.Close()
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (h *Hub) BroadcastStock(stock model.StockResponse) {
	select {
	case h.broadcast <- stock:
	default:
	}
}
