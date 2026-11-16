package routes

import (
	"todo_api/internal/config"
	"todo_api/internal/handlers"
	"todo_api/internal/stream"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterAuctionRoutes 只負責 Auction Domain 的路由註冊。
func RegisterAuctionRoutes(
	router *gin.Engine,
	pool *pgxpool.Pool,
	cfg *config.Config,
	auctionHub *stream.AuctionEventHub,
) {
	// 建立競拍商品
	router.POST(
		"/auctions",
		handlers.CreateAuctionListingHandler(pool, cfg),
	)

	// 查詢所有競拍商品
	router.GET(
		"/auctions",
		handlers.GetAuctionListingsHandler(pool),
	)

	// 查詢單一競拍商品
	router.GET(
		"/auctions/:id",
		handlers.GetAuctionListingByIDHandler(pool),
	)

	// 對指定商品出價
	router.POST(
		"/auctions/:id/bids",
		handlers.PlaceBidHandler(pool, cfg),
	)

	// 查詢出價紀錄
	router.GET(
		"/auctions/:id/bids",
		handlers.GetAuctionBidsHandler(pool),
	)

	// SSE：訂閱指定拍賣的即時事件
	router.GET(
		"/auctions/:id/stream",
		handlers.AuctionSseHandler(auctionHub),
	)
}
