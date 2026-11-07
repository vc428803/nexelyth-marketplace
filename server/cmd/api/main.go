// responsible for running database
package main

import (
	"context"
	"log"
	"todo_api/internal/app"
	"todo_api/internal/config"
	"todo_api/internal/database"
	"todo_api/internal/worker"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// 載入設定
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	// 建立資料庫連線池
	var pool *pgxpool.Pool
	pool, err = database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	// 啟動拍賣自動結算背景工作
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()

	go worker.StartAuctionWorker(workerCtx, pool)

	// 建立已完成基礎設定的 router
	router := app.NewRouter(cfg)

	// 初始化背景服務
	chatRoom := app.InitChatRoom()

	const maxProductsForStream = 10
	productsCache, cr := app.InitProductsStreamServices(pool, maxProductsForStream)
	defer cr.Stop()

	// 統一註冊所有 routes
	app.RegisterRoutes(router, pool, cfg, chatRoom, productsCache)

	// 啟動 server
	log.Printf("server starting on port %s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
