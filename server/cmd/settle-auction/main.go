package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"todo_api/internal/config"
	"todo_api/internal/database"
	"todo_api/internal/repository"
)

func main() {
	listingID := flag.String(
		"listing",
		"",
		"UUID of the auction listing to settle",
	)

	execute := flag.Bool(
		"execute",
		false,
		"Execute settlement transaction",
	)

	flag.Parse()

	if *listingID == "" {
		log.Fatal("missing required -listing argument")
	}

	if !*execute {
		fmt.Println("DRY RUN: settlement was NOT executed")
		fmt.Println("Listing:", *listingID)
		fmt.Println("Add -execute to perform settlement")
		return
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	pool, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	// 防止誤用：只有明確指定 -execute 才執行。
	// 請勿將此工具暴露為公開 HTTP API。
	ctx := context.Background()

	if err := repository.SettleAuction(
		ctx,
		pool,
		*listingID,
	); err != nil {
		log.Printf("Settlement failed: %v", err)
		os.Exit(1)
	}

	fmt.Println("Settlement completed successfully")
}
