package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"todo_api/internal/models"
	"todo_api/internal/utils"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrAuctionItemNotOwned    = errors.New("auction item not owned")
	ErrAuctionItemUnavailable = errors.New("auction item unavailable")
)

// 建立一筆新的拍賣商品
func CreateAuctionListing(
	pool *pgxpool.Pool,
	sellerID string,
	req models.CreateAuctionListingRequest,
) (*models.AuctionListing, error) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// 1. 鎖定背包中的實際物品
	var ownerID string
	var itemName string
	var imageURL string
	var description string
	var category string
	var isTradeable bool
	var status string

	err = tx.QueryRow(ctx, `
        SELECT
            i.owner_id,
            d.name,
            COALESCE(d.image_url, ''),
            COALESCE(d.description, ''),
            d.category,
            d.is_tradeable,
            i.status
        FROM inventory_items i
        JOIN item_definitions d
            ON d.id = i.item_definition_id
        WHERE i.id = $1
        FOR UPDATE OF i
    `, req.InventoryItemID).Scan(
		&ownerID,
		&itemName,
		&imageURL,
		&description,
		&category,
		&isTradeable,
		&status,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAuctionItemNotOwned
	}
	if err != nil {
		return nil, err
	}

	// 2. 驗證物品所有權與狀態
	if ownerID != sellerID {
		return nil, ErrAuctionItemNotOwned
	}

	if !isTradeable || status != "AVAILABLE" {
		return nil, ErrAuctionItemUnavailable
	}

	// 3. 從資料庫資料建立競拍紀錄
	var listing models.AuctionListing

	err = tx.QueryRow(ctx, `
        INSERT INTO auction_listings (
            seller_id,
            inventory_item_id,
            item_name,
            item_image_url,
            category,
            item_description,
            starting_price,
            current_price,
            min_increment,
            status,
            start_time,
            end_time
        )
        VALUES (
            $1, $2, $3, $4, $5, $6,
            $7, $7, $8, 'active', NOW(), $9
        )
        RETURNING
            id,
            seller_id,
            item_name,
            item_image_url,
            category,
            COALESCE(item_description, ''),
            starting_price,
            current_price,
            min_increment,
            highest_bidder_id,
            status,
            start_time,
            end_time,
            created_at,
            updated_at
    `,
		sellerID,
		req.InventoryItemID,
		itemName,
		imageURL,
		category,
		description,
		req.StartingPrice,
		req.MinIncrement,
		req.EndTime,
	).Scan(
		&listing.ID,
		&listing.SellerID,
		&listing.ItemName,
		&listing.ItemImageURL,
		&listing.Category,
		&listing.ItemDescription,
		&listing.StartingPrice,
		&listing.CurrentPrice,
		&listing.MinIncrement,
		&listing.HighestBidderID,
		&listing.Status,
		&listing.StartTime,
		&listing.EndTime,
		&listing.CreatedAt,
		&listing.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// 4. 將物品標記為已上架
	commandTag, err := tx.Exec(ctx, `
        UPDATE inventory_items
        SET status = 'LISTED',
            updated_at = NOW()
        WHERE id = $1
          AND owner_id = $2
          AND status = 'AVAILABLE'
    `, req.InventoryItemID, sellerID)

	if err != nil {
		return nil, err
	}

	if commandTag.RowsAffected() != 1 {
		return nil, ErrAuctionItemUnavailable
	}

	// 5. 全部成功才提交
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &listing, nil
}

// GetAuctionListings 取得所有拍賣商品
// 目前先只抓 active，並依建立時間新到舊排序
func GetAuctionListings(pool *pgxpool.Pool) ([]models.AuctionListing, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	utils.PerformOperation(ctx)

	query := `
		SELECT
			a.id,
			a.seller_id,
			a.item_name,
			COALESCE(d.image_url, '') AS item_image_url,
			a.item_description,
			a.starting_price,
			a.current_price,
			a.min_increment,
			a.highest_bidder_id,
			a.status,
			a.start_time,
			a.end_time,
			a.created_at,
			a.updated_at
		FROM auction_listings a
		LEFT JOIN inventory_items i
			ON a.inventory_item_id = i.id
		LEFT JOIN item_definitions d
			ON i.item_definition_id = d.id
		WHERE a.status = 'active'
		ORDER BY a.created_at DESC
		`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var listings []models.AuctionListing

	for rows.Next() {
		var listing models.AuctionListing

		err := rows.Scan(
			&listing.ID,
			&listing.SellerID,
			&listing.ItemName,
			&listing.ItemImageURL,
			&listing.ItemDescription,
			&listing.StartingPrice,
			&listing.CurrentPrice,
			&listing.MinIncrement,
			&listing.HighestBidderID,
			&listing.Status,
			&listing.StartTime,
			&listing.EndTime,
			&listing.CreatedAt,
			&listing.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		listings = append(listings, listing)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return listings, nil
}

// 根據 id 取得單一拍賣商品
func GetAuctionListingByID(pool *pgxpool.Pool, listingID string) (*models.AuctionListing, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	utils.PerformOperation(ctx)

	query := `
		SELECT
			id,
			seller_id,
			item_name,
			item_image_url,
			category,
			item_description,
			starting_price,
			current_price,
			min_increment,
			highest_bidder_id,
			status,
			start_time,
			end_time,
			created_at,
			updated_at
		FROM auction_listings
		WHERE id = $1
	`

	var listing models.AuctionListing

	err := pool.QueryRow(ctx, query, listingID).Scan(
		&listing.ID,
		&listing.SellerID,
		&listing.ItemName,
		&listing.ItemImageURL,
		&listing.Category,
		&listing.ItemDescription,
		&listing.StartingPrice,
		&listing.CurrentPrice,
		&listing.MinIncrement,
		&listing.HighestBidderID,
		&listing.Status,
		&listing.StartTime,
		&listing.EndTime,
		&listing.CreatedAt,
		&listing.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &listing, nil
}

// GetAuctionBidsByListingID 取得某一筆拍賣商品的所有出價紀錄
func GetAuctionBidsByListingID(pool *pgxpool.Pool, listingID string) ([]models.AuctionBid, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	utils.PerformOperation(ctx)

	query := `
		SELECT
			id,
			listing_id,
			bidder_id,
			bid_amount,
			created_at
		FROM auction_bids
		WHERE listing_id = $1
		ORDER BY created_at DESC
	`

	rows, err := pool.Query(ctx, query, listingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bids []models.AuctionBid

	for rows.Next() {
		var bid models.AuctionBid

		err := rows.Scan(
			&bid.ID,
			&bid.ListingID,
			&bid.BidderID,
			&bid.BidAmount,
			&bid.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		bids = append(bids, bid)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return bids, nil
}

// PlaceBid 建立一筆出價，並同步更新 auction_listings 的 current_price / highest_bidder_id
// 這裡一定要用 transaction，避免多人同時出價造成資料不一致
func PlaceBid(pool *pgxpool.Pool, listingID string, bidderID string, bidAmount int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if bidAmount <= 0 {
		return errors.New("bid amount must be positive")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. 鎖定拍賣商品
	var sellerID string
	var highestBidder sql.NullString
	var currentPrice, minIncrement int64
	var status string
	var endTime time.Time

	err = tx.QueryRow(ctx, `
        SELECT seller_id, highest_bidder_id,
               current_price, min_increment,
               status, end_time
        FROM auction_listings
        WHERE id = $1
        FOR UPDATE
    `, listingID).Scan(
		&sellerID, &highestBidder,
		&currentPrice, &minIncrement,
		&status, &endTime,
	)
	if err != nil {
		return fmt.Errorf("load listing: %w", err)
	}

	if sellerID == bidderID {
		return errors.New("seller cannot bid on own listing")
	}
	if status != "active" {
		return errors.New("listing is not active")
	}
	if !time.Now().Before(endTime) {
		return errors.New("auction has already ended")
	}
	if currentPrice < 0 || minIncrement <= 0 {
		return errors.New("invalid auction price configuration")
	}
	// 使用減法比較，避免 currentPrice + minIncrement 溢位
	if bidAmount <= currentPrice ||
		bidAmount-currentPrice < minIncrement {
		return errors.New("bid amount is too low")
	}

	// 2. 取得前任最高出價者的保留資金
	var oldHoldID, oldBidID string
	var oldAmount int64
	var oldBidderID string
	hasOldHold := highestBidder.Valid

	if hasOldHold {
		err = tx.QueryRow(ctx, `
            SELECT id, bid_id, bidder_id, amount
            FROM auction_fund_holds
            WHERE listing_id = $1 AND status = 'HELD'
            FOR UPDATE
        `, listingID).Scan(
			&oldHoldID, &oldBidID,
			&oldBidderID, &oldAmount,
		)
		if err != nil {
			return fmt.Errorf("existing bid hold missing: %w", err)
		}

		if oldBidderID != highestBidder.String ||
			oldAmount != currentPrice {
			return errors.New("existing bid hold is inconsistent")
		}
	}

	// 3. 按固定順序鎖定涉及的錢包，降低死鎖風險
	userIDs := []string{bidderID}
	if hasOldHold && oldBidderID != bidderID {
		userIDs = append(userIDs, oldBidderID)
	}
	sort.Strings(userIDs)

	type lockedWallet struct {
		ID      string
		Balance int64
	}
	wallets := make(map[string]*lockedWallet)

	for _, userID := range userIDs {
		w := &lockedWallet{}
		err = tx.QueryRow(ctx, `
            SELECT id, balance
            FROM wallets
            WHERE user_id = $1 AND currency_code = 'G'
            FOR UPDATE
        `, userID).Scan(&w.ID, &w.Balance)
		if err != nil {
			return fmt.Errorf("wallet unavailable for user %s: %w", userID, err)
		}
		wallets[userID] = w
	}

	// 4. 計算資金是否足夠
	// 自己提高出價時，原有保留金額可以重新使用
	available := wallets[bidderID].Balance
	if hasOldHold && oldBidderID == bidderID {
		// oldAmount <= currentPrice < bidAmount
		// 所以只需檢查增加的差額
		if available < bidAmount-oldAmount {
			return errors.New("insufficient wallet balance")
		}
	} else if available < bidAmount {
		return errors.New("insufficient wallet balance")
	}

	// 5. 釋放前任最高出價者的資金
	if hasOldHold {
		oldWallet := wallets[oldBidderID]
		const maxInt64 = int64(^uint64(0) >> 1)

		// 防止退款時 BIGINT 溢位
		if oldAmount <= 0 ||
			oldWallet.Balance > maxInt64-oldAmount {
			return errors.New("invalid refund amount")
		}

		oldWallet.Balance += oldAmount

		_, err = tx.Exec(ctx, `
            UPDATE wallets
            SET balance = $1, updated_at = NOW()
            WHERE id = $2
        `, oldWallet.Balance, oldWallet.ID)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
            INSERT INTO wallet_transactions
                (wallet_id, amount_delta, balance_after,
                 transaction_type, reference_id)
            VALUES ($1, $2, $3, 'BID_RELEASE', $4)
        `, oldWallet.ID, oldAmount,
			oldWallet.Balance, oldBidID)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
            UPDATE auction_fund_holds
            SET status = 'RELEASED', updated_at = NOW()
            WHERE id = $1 AND status = 'HELD'
        `, oldHoldID)
		if err != nil {
			return err
		}
	}

	// 6. 保留新出價者的資金
	newWallet := wallets[bidderID]

	if newWallet.Balance < bidAmount {
		return errors.New("insufficient wallet balance")
	}

	newWallet.Balance -= bidAmount

	_, err = tx.Exec(ctx, `
        UPDATE wallets
        SET balance = $1, updated_at = NOW()
        WHERE id = $2
    `, newWallet.Balance, newWallet.ID)
	if err != nil {
		return err
	}

	// 7. 新增競標紀錄，取得 bid ID
	var newBidID string

	err = tx.QueryRow(ctx, `
        INSERT INTO auction_bids
            (listing_id, bidder_id, bid_amount)
        VALUES ($1, $2, $3)
        RETURNING id
    `, listingID, bidderID, bidAmount).Scan(&newBidID)
	if err != nil {
		return err
	}

	// 8. 建立新資金保留紀錄
	_, err = tx.Exec(ctx, `
        INSERT INTO auction_fund_holds
            (listing_id, bid_id, bidder_id, amount, status)
        VALUES ($1, $2, $3, $4, 'HELD')
    `, listingID, newBidID, bidderID, bidAmount)
	if err != nil {
		return err
	}

	// 9. 記錄錢包扣款
	_, err = tx.Exec(ctx, `
        INSERT INTO wallet_transactions
            (wallet_id, amount_delta, balance_after,
             transaction_type, reference_id)
        VALUES ($1, $2, $3, 'BID_HOLD', $4)
    `, newWallet.ID, -bidAmount,
		newWallet.Balance, newBidID)
	if err != nil {
		return err
	}

	// 10. 更新最高出價
	_, err = tx.Exec(ctx, `
        UPDATE auction_listings
        SET current_price = $1,
            highest_bidder_id = $2,
            updated_at = NOW()
        WHERE id = $3
    `, bidAmount, bidderID, listingID)
	if err != nil {
		return err
	}

	// 11. 所有資料一次提交
	return tx.Commit(ctx)
}

// 已完成結算的拍賣再次呼叫時，不會重複付款或交付物品。
func SettleAuction(
	ctx context.Context,
	pool *pgxpool.Pool,
	listingID string,
) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. 鎖定拍賣，避免重複結算或同時競標
	var sellerID string
	var highestBidder sql.NullString
	var inventoryItemID sql.NullString
	var currentPrice int64
	var status string
	var endTime time.Time

	err = tx.QueryRow(ctx, `
         SELECT seller_id, highest_bidder_id,
                inventory_item_id, current_price,
                status, end_time
         FROM auction_listings
         WHERE id = $1
         FOR UPDATE
     `, listingID).Scan(
		&sellerID, &highestBidder,
		&inventoryItemID, &currentPrice,
		&status, &endTime,
	)
	if err != nil {
		return fmt.Errorf("load auction: %w", err)
	}

	// 已成交或已流標的拍賣，不再重複執行結算
	if status == "completed" || status == "expired" {
		return nil
	}

	if status != "active" {
		return errors.New("auction is not active")
	}

	if time.Now().Before(endTime) {
		return errors.New("auction has not ended")
	}

	// 無論成交或流標，都必須有對應的背包物品。
	if !inventoryItemID.Valid {
		return errors.New("auction inventory item is missing")
	}

	// 流標前，確認沒有任何仍被保留的競標資金
	var activeHoldCount int

	err = tx.QueryRow(ctx, `
    SELECT COUNT(*)
    FROM auction_fund_holds
    WHERE listing_id = $1
      AND status = 'HELD'
`, listingID).Scan(&activeHoldCount)

	if err != nil {
		return fmt.Errorf("check expired auction holds: %w", err)
	}

	if activeHoldCount > 0 {
		return errors.New("cannot expire auction with active fund holds")
	}

	// 無人出價：將拍賣標記為流標，並解除物品上架狀態。
	if !highestBidder.Valid {
		// 鎖定物品，驗證目前仍屬賣家且處於上架狀態。
		var itemOwnerID, itemStatus string

		err = tx.QueryRow(ctx, `
			SELECT owner_id, status
			FROM inventory_items
			WHERE id = $1
			FOR UPDATE
		`, inventoryItemID.String).Scan(
			&itemOwnerID, &itemStatus,
		)
		if err != nil {
			return fmt.Errorf("load expired auction item: %w", err)
		}

		if itemOwnerID != sellerID || itemStatus != "LISTED" {
			return errors.New("expired auction item ownership or status mismatch")
		}

		// 還原物品為可用狀態，不改變所有權。
		tag, err := tx.Exec(ctx, `
			UPDATE inventory_items
			SET status = 'AVAILABLE',
				updated_at = NOW()
			WHERE id = $1
			  AND owner_id = $2
			  AND status = 'LISTED'
		`, inventoryItemID.String, sellerID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return errors.New("failed to release expired auction item")
		}

		// 將拍賣標記為流標。
		tag, err = tx.Exec(ctx, `
			UPDATE auction_listings
			SET status = 'expired',
				updated_at = NOW()
			WHERE id = $1
			  AND status = 'active'
		`, listingID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return errors.New("failed to expire auction")
		}

		// 物品解除上架及拍賣狀態更新，一起提交。
		return tx.Commit(ctx)
	}

	buyerID := highestBidder.String

	if buyerID == sellerID {
		return errors.New("seller cannot be winning bidder")
	}

	// 2. 鎖定保留資金並驗證金額
	var holdID string
	var holdBidID string
	var holdAmount int64
	var holdBidderID string

	err = tx.QueryRow(ctx, `
         SELECT id, bid_id, bidder_id, amount
         FROM auction_fund_holds
         WHERE listing_id = $1
           AND status = 'HELD'
         FOR UPDATE
     `, listingID).Scan(
		&holdID, &holdBidID,
		&holdBidderID, &holdAmount,
	)
	if err != nil {
		return fmt.Errorf("load fund hold: %w", err)
	}

	if holdBidderID != buyerID ||
		holdAmount != currentPrice ||
		holdAmount <= 0 {
		return errors.New("auction fund hold mismatch")
	}

	// 3. 鎖定物品並確認仍屬賣家
	var itemOwnerID, itemStatus string

	err = tx.QueryRow(ctx, `
         SELECT owner_id, status
         FROM inventory_items
         WHERE id = $1
         FOR UPDATE
     `, inventoryItemID.String).Scan(
		&itemOwnerID, &itemStatus,
	)
	if err != nil {
		return fmt.Errorf("load inventory item: %w", err)
	}

	if itemOwnerID != sellerID || itemStatus != "LISTED" {
		return errors.New("inventory ownership or status mismatch")
	}

	// 4. 鎖定賣家錢包
	var sellerWalletID string
	var sellerBalance int64

	err = tx.QueryRow(ctx, `
         SELECT id, balance
         FROM wallets
         WHERE user_id = $1
           AND currency_code = 'G'
         FOR UPDATE
     `, sellerID).Scan(&sellerWalletID, &sellerBalance)
	if err != nil {
		return fmt.Errorf("load seller wallet: %w", err)
	}

	// 避免 BIGINT 溢位
	const maxInt64 = int64(^uint64(0) >> 1)

	if sellerBalance > maxInt64-holdAmount {
		return errors.New("seller wallet balance overflow")
	}

	newBalance := sellerBalance + holdAmount

	// 5. 將已保留款項支付給賣家
	tag, err := tx.Exec(ctx, `
         UPDATE wallets
         SET balance = $1,
             updated_at = NOW()
         WHERE id = $2
     `, newBalance, sellerWalletID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("seller wallet update failed")
	}

	// 6. 建立賣家收入流水
	_, err = tx.Exec(ctx, `
         INSERT INTO wallet_transactions
             (wallet_id, amount_delta, balance_after,
              transaction_type, reference_id)
         VALUES ($1, $2, $3, 'SALE', $4)
     `, sellerWalletID, holdAmount, newBalance, holdBidID)
	if err != nil {
		return err
	}

	// 7. 將保留資金標記為已結算
	tag, err = tx.Exec(ctx, `
         UPDATE auction_fund_holds
         SET status = 'SETTLED',
             updated_at = NOW()
         WHERE id = $1
           AND status = 'HELD'
     `, holdID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("fund hold settlement failed")
	}

	// 8. 轉移物品所有權給得標者
	tag, err = tx.Exec(ctx, `
         UPDATE inventory_items
         SET owner_id = $1,
             status = 'AVAILABLE',
             updated_at = NOW()
         WHERE id = $2
           AND owner_id = $3
           AND status = 'LISTED'
     `, buyerID, inventoryItemID.String, sellerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("inventory transfer failed")
	}

	// 9. 將拍賣標記為完成
	tag, err = tx.Exec(ctx, `
         UPDATE auction_listings
         SET status = 'completed',
             updated_at = NOW()
         WHERE id = $1
           AND status = 'active'
     `, listingID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("auction completion failed")
	}

	// 10. 全部成功才正式提交
	return tx.Commit(ctx)
}
