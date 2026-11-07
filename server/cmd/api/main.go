package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"todo_api/internal/app"
	"todo_api/internal/config"
	"todo_api/internal/database"
	"todo_api/internal/worker"
)

func main() {
	// ========================================
	// 1. 載入環境設定
	// ========================================
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// ========================================
	// 2. 建立 PostgreSQL 連線池
	// ========================================
	pool, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// 最後關閉連線池
	defer pool.Close()

	// ========================================
	// 3. 建立可接收系統停止訊號的 Context
	// ========================================
	serverCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// ========================================
	// 4. 啟動 Auction Background Worker
	// ========================================

	// Worker 使用獨立的 Context，
	// 方便在 HTTP Server 關閉後通知 Worker 停止。
	workerCtx, cancelWorker := context.WithCancel(
		context.Background(),
	)

	var workerWG sync.WaitGroup
	workerWG.Add(1)

	go func() {
		defer workerWG.Done()

		log.Println("Auction worker starting")

		worker.StartAuctionWorker(
			workerCtx,
			pool,
		)

		log.Println("Auction worker exited")
	}()

	// defer 為後進先出（LIFO）。
	// 此清理會早於 pool.Close() 執行。
	defer func() {
		log.Println("Stopping auction worker...")

		// 通知 Worker 停止
		cancelWorker()

		// 等待 Worker 真正退出
		workerWG.Wait()

		log.Println("Auction worker stopped")
	}()

	// ========================================
	// 5. 初始化 Gin Router
	// ========================================
	router := app.NewRouter(cfg)

	// ========================================
	// 6. 初始化既有背景服務
	// ========================================
	chatRoom := app.InitChatRoom()

	const maxProductsForStream = 10

	productsCache, cr := app.InitProductsStreamServices(
		pool,
		maxProductsForStream,
	)

	// 保留既有背景服務清理
	defer cr.Stop()

	// ========================================
	// 7. 註冊所有 API Routes
	// ========================================
	app.RegisterRoutes(
		router,
		pool,
		cfg,
		chatRoom,
		productsCache,
	)

	// ========================================
	// 8. 建立 HTTP Server
	// ========================================
	httpServer := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,

		// 避免客戶端長時間不送完 HTTP Headers
		ReadHeaderTimeout: 5 * time.Second,

		IdleTimeout: 60 * time.Second,

		// 未設定全域 WriteTimeout：
		// 避免影響專案中的 SSE 長連線。
	}

	// 使用 buffered channel，
	// 避免 Server Goroutine 回報錯誤時被阻塞。
	serverErrors := make(chan error, 1)

	// ========================================
	// 9. 啟動 HTTP Server
	// ========================================
	go func() {
		err := httpServer.ListenAndServe()

		if err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	log.Printf(
		"HTTP server starting on port %s",
		cfg.Port,
	)

	// ========================================
	// 10. 等待停止訊號或 HTTP 啟動錯誤
	// ========================================
	select {
	case <-serverCtx.Done():
		log.Println("Shutdown signal received")

	case err := <-serverErrors:
		log.Printf("HTTP server error: %v", err)
	}

	// ========================================
	// 11. Graceful Shutdown
	// ========================================

	// 停止接收新 HTTP 請求，
	// 並等待現有請求完成。
	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancelShutdown()

	log.Println("Shutting down HTTP server...")
	// http.Server.Shutdown() 不會自動等待所有已接管的長連線（例如 WebSocket）
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf(
			"HTTP server shutdown error: %v",
			err,
		)
	}

	log.Println("HTTP server stopped")

	// ========================================
	// 12. main() 返回後執行 defer
	// ========================================

	// 實際清理順序：
	//
	// 1. cancelShutdown()
	// 2. cr.Stop()
	// 3. cancelWorker()
	// 4. workerWG.Wait()
	// 5. stop()
	// 6. pool.Close()
}
