package worker

import (
	"context"
	"log/slog"
	"time"

	"todo_api/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

// StartAuctionWorker 啟動拍賣結算背景工作。
func StartAuctionWorker(
	ctx context.Context,
	pool *pgxpool.Pool,
) {
	// 啟動後立即補處理逾期拍賣
	checkExpiredAuctions(ctx, pool)

	// 每 30 分鐘執行一次保底檢查
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Auction worker stopped")
			return

		case <-ticker.C:
			checkExpiredAuctions(ctx, pool)
		}
	}
}

// checkExpiredAuctions 尋找已截止但尚未結算的拍賣。
func checkExpiredAuctions(
	ctx context.Context,
	pool *pgxpool.Pool,
) {
	queryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)

	rows, err := pool.Query(queryCtx, `
        SELECT id
        FROM auction_listings
        WHERE status = 'active'
          AND end_time <= NOW()
        ORDER BY end_time
        LIMIT 100
    `)
	if err != nil {
		cancel()
		slog.Error("Failed to query expired auctions", "error", err)
		return
	}

	var listingIDs []string

	for rows.Next() {
		var id string

		if err := rows.Scan(&id); err != nil {
			slog.Error("Failed to scan auction ID", "error", err)
			continue
		}

		listingIDs = append(listingIDs, id)
	}

	if err := rows.Err(); err != nil {
		slog.Error("Error reading expired auctions", "error", err)
	}

	rows.Close()
	cancel()

	if len(listingIDs) == 0 {
		slog.Info("No expired auctions to settle")
		return
	}

	slog.Info("Found expired auctions", "count", len(listingIDs))

	for _, listingID := range listingIDs {
		if err := repository.SettleAuction(
			ctx,
			pool,
			listingID,
		); err != nil {
			slog.Error(
				"Auction settlement failed",
				"listingID", listingID,
				"error", err,
			)
			continue
		}

		slog.Info(
			"Auction settlement completed",
			"listingID", listingID,
		)
	}
}
