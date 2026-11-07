package worker

import (
	"context"
	"log/slog"
	"time"

	"todo_api/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// 每 30 分鐘進行一次保底檢查
	auctionCheckInterval = 30 * time.Minute

	// 單次查詢逾時
	auctionQueryTimeout = 15 * time.Second

	// 每次最多取出 100 筆待結算拍賣
	auctionBatchSize = 100
)

// StartAuctionWorker 啟動拍賣結算背景工作。
//
// 執行流程：
// 1. 啟動時立即檢查逾期拍賣
// 2. 每 30 分鐘執行保底檢查
// 3. 收到 ctx.Done() 後停止
func StartAuctionWorker(
	ctx context.Context,
	pool *pgxpool.Pool,
) {
	slog.Info(
		"Auction worker started",
		"interval", auctionCheckInterval.String(),
	)

	// ========================================
	// 1. 啟動立即補結算
	// ========================================
	if ctx.Err() != nil {
		return
	}

	checkExpiredAuctions(ctx, pool)

	// ========================================
	// 2. 建立定期檢查 Ticker
	// ========================================
	ticker := time.NewTicker(auctionCheckInterval)
	defer ticker.Stop()

	// ========================================
	// 3. 持續等待排程或取消訊號
	// ========================================
	for {
		select {
		case <-ctx.Done():
			slog.Info("Auction worker cancelled")
			return

		case <-ticker.C:
			// 避免取消訊號與 Ticker 同時觸發時，
			// 又開始新的資料庫查詢。
			if ctx.Err() != nil {
				return
			}

			checkExpiredAuctions(ctx, pool)
		}
	}
}

// checkExpiredAuctions 查詢已截止但尚未結算的拍賣。
func checkExpiredAuctions(
	ctx context.Context,
	pool *pgxpool.Pool,
) {
	// 若已收到停止訊號，不再查詢資料庫。
	if ctx.Err() != nil {
		return
	}

	slog.Info("Checking expired auctions")

	// ========================================
	// 1. 限制資料庫查詢時間
	// ========================================
	queryCtx, cancel := context.WithTimeout(
		ctx,
		auctionQueryTimeout,
	)
	defer cancel()

	// ========================================
	// 2. 查詢待結算拍賣
	// ========================================
	rows, err := pool.Query(queryCtx, `
		SELECT id
		FROM auction_listings
		WHERE status = 'active'
		  AND end_time <= NOW()
		ORDER BY end_time, id
		LIMIT $1
	`, auctionBatchSize)

	if err != nil {
		if ctx.Err() != nil {
			return
		}

		slog.Error(
			"Failed to query expired auctions",
			"error", err,
		)
		return
	}

	// ========================================
	// 3. 收集待結算拍賣 ID
	// ========================================
	var listingIDs []string

	for rows.Next() {
		var listingID string

		if err := rows.Scan(&listingID); err != nil {
			slog.Error(
				"Failed to scan auction ID",
				"error", err,
			)

			rows.Close()
			return
		}

		listingIDs = append(
			listingIDs,
			listingID,
		)
	}

	// 檢查查詢結果是否完整讀取
	if err := rows.Err(); err != nil {
		rows.Close()

		if ctx.Err() != nil {
			return
		}

		slog.Error(
			"Failed to read expired auctions",
			"error", err,
		)
		return
	}

	// ========================================
	// 4. 關閉 Rows，釋放資料庫連線
	// ========================================
	rows.Close()

	// Query Context 已不再需要
	cancel()

	if ctx.Err() != nil {
		return
	}

	// ========================================
	// 5. 沒有待結算拍賣
	// ========================================
	if len(listingIDs) == 0 {
		slog.Info("No expired auctions to settle")
		return
	}

	slog.Info(
		"Expired auctions found",
		"count", len(listingIDs),
	)

	// ========================================
	// 6. 逐筆執行結算
	// ========================================
	successCount := 0
	failedCount := 0

	for _, listingID := range listingIDs {

		// 每筆結算前檢查取消訊號
		if ctx.Err() != nil {
			slog.Info(
				"Auction settlement batch cancelled",
				"success", successCount,
				"failed", failedCount,
			)
			return
		}

		// 重用已完成的 SettleAuction()
		err := repository.SettleAuction(
			ctx,
			pool,
			listingID,
		)

		if err != nil {
			// 關閉時取消的操作不視為一般失敗
			if ctx.Err() != nil {
				slog.Info(
					"Auction settlement interrupted",
					"listingID", listingID,
				)
				return
			}

			failedCount++

			slog.Error(
				"Auction settlement failed",
				"listingID", listingID,
				"error", err,
			)

			// 單筆失敗不阻止其他拍賣結算
			continue
		}

		successCount++

		slog.Info(
			"Auction settlement completed",
			"listingID", listingID,
		)
	}

	// ========================================
	// 7. 輸出本次執行結果
	// ========================================
	slog.Info(
		"Auction settlement batch finished",
		"total", len(listingIDs),
		"success", successCount,
		"failed", failedCount,
	)
}
