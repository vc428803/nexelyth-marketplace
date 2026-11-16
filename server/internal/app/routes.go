package app

// app 去 import routes
import (
	approutes "todo_api/internal/app/routes"
	"todo_api/internal/chat"
	"todo_api/internal/config"
	"todo_api/internal/stream"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 這裡只保留總調度，不放各模組細節
func RegisterRoutes(
	router *gin.Engine,
	pool *pgxpool.Pool,
	cfg *config.Config,
	chatRoom *chat.Room,
	productsCache *stream.ProductsCache,
	auctionHub *stream.AuctionEventHub, // 新增：拍賣事件中心
) {
	// 基礎健康檢查 / 根路由
	approutes.RegisterHealthRoutes(router)

	// Todo 模組
	approutes.RegisterTodoRoutes(router, pool)

	// WebSocket 模組
	approutes.RegisterWebSocketRoutes(router, chatRoom)

	// Auth 模組
	approutes.RegisterAuthRoutes(router, pool, cfg)

	// Wallet 模組
	approutes.RegisterWalletRoutes(router, pool, cfg)

	// Marketplace Inventory 模組
	approutes.RegisterInventoryRoutes(router, pool, cfg)

	// Product 模組
	approutes.RegisterProductRoutes(router, pool, productsCache)

	// User 模組
	approutes.RegisterUserRoutes(router, pool)

	// Auction 模組
	approutes.RegisterAuctionRoutes(router, pool, cfg, auctionHub)
	// Admin 模組
	approutes.RegisterAdminRoutes(router, pool, cfg)

	// 積分獎勵模組
	approutes.RegisterEventRoutes(router, pool, cfg)
}
