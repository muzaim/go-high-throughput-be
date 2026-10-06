package routes

import (
	"indico-test-be/internal/handler"
	"indico-test-be/internal/middleware"
	ws "indico-test-be/internal/websocket"

	"github.com/gin-gonic/gin"
)

func SetupRouter(inventoryHandler *handler.InventoryHandler, hub *ws.Hub) *gin.Engine {
	r := gin.New()

	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	r.StaticFile("/swagger.json", "./docs/openapi.json")

	r.GET("/docs", func(c *gin.Context) {
		html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Flash Sale Inventory Reservation API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: '/swagger.json',
        dom_id: '#swagger-ui',
      });
    };
  </script>
</body>
</html>`
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(200, html)
	})

	if hub != nil {
		r.GET("/ws/stock", hub.HandleWS)
	}

	api := r.Group("/api/v1/inventory")
	{
		api.GET("/items", inventoryHandler.GetAllItems)
		api.GET("/items/:id", inventoryHandler.GetItemDetail)
		api.POST("/reserve", inventoryHandler.Reserve)
		api.POST("/confirm", inventoryHandler.Confirm)
		api.GET("/stock", inventoryHandler.GetStock)
	}

	return r
}
