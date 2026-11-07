/*
資料庫連接池預先維持少量持久連接（如 10–20 個），請求時直接借用、用完自動歸還，避免重複建立/關閉連接的開銷。
在高併發場景下（如每秒百筆請求），可大幅降低延遲、節省資源，提升系統效能與穩定性。
*/
package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// contoling database to our context
// Create the connection pool
// db, err := NewPg(rootCtx, dbConfig, WithPgxConfig(dbConfig))

type DBConfig struct {
	UserName string
	Password string
	Host     string
	Port     int
	DBName   string
}

// Advanced Configuration (More flexible, customizable settings)
// Creating the Connection Pool => https://resources.hexacluster.ai/blog/postgresql/postgresql-client-side-connection-pooling-in-golang-using-pgxpool/
func Connect(databaseURL string) (*pgxpool.Pool, error) {

	ctx := context.Background()

	// 解析設定
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		slog.Error("Unable to parse pool config", slog.String("error", err.Error()))
		return nil, err
	}

	// 自訂連線池參數
	config.MaxConns = 20                        // 最大連線數
	config.MinConns = 0                         // Go Server 在沒有資料庫請求時，就不必保留最低數量的連線，不代表 Neon 一定會在 2 分鐘後休眠。Neon 的休眠由其自身機制決定
	config.HealthCheckPeriod = 30 * time.Second // 健康檢查週期
	config.MaxConnLifetime = 1 * time.Hour      // 連線最長存活時間
	config.MaxConnIdleTime = 2 * time.Minute    // 閒置連線達到約 2 分鐘後，可以由連線池的維護機制回收

	var pool *pgxpool.Pool
	// 建立連線池
	pool, err = pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		slog.Error("Unable to create advanced connection pool", slog.String("error", err.Error()))
		return nil, err
	}

	// 驗證連線
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	slog.Info("Successfully connected to PostgreSQL database")
	return pool, nil
}
