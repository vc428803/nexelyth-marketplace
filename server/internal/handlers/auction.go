package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"time"
	"todo_api/internal/config"
	"todo_api/internal/models"
	"todo_api/internal/repository"
	"todo_api/internal/stream"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 從 cookie 裡解析 JWT，拿出目前登入者的 user_id
func getUserIDFromCookie(c *gin.Context, cfg *config.Config) (string, error) {
	tokenString, err := c.Cookie("token")
	if err != nil {
		return "", err
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.JWTSecret), nil
	})

	if err != nil {
		return "", err
	}

	// 未登入或 JWT 資料不正確的使用者就不會因為空錯誤而通過驗證
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", errors.New("invalid token claims")
	}

	userIDValue, ok := claims["user_id"]
	if !ok {
		return "", errors.New("missing user_id claim")
	}

	userID, ok := userIDValue.(string)
	if !ok || userID == "" {
		return "", errors.New("invalid user_id claim")
	}

	return userID, nil
}

// 建立拍賣商品

func CreateAuctionListingHandler(
	pool *pgxpool.Pool,
	cfg *config.Config,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := getUserIDFromCookie(c, cfg)

		if err != nil || userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized",
			})
			return
		}

		var req models.CreateAuctionListingRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid auction request",
			})
			return
		}

		if !req.EndTime.After(time.Now()) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "end_time must be in the future",
			})
			return
		}

		listing, err := repository.CreateAuctionListing(
			pool,
			userID,
			req,
		)

		if err != nil {
			switch {
			case errors.Is(err, repository.ErrAuctionItemNotOwned):
				c.JSON(http.StatusForbidden, gin.H{
					"error": "item is not owned by current user",
				})

			case errors.Is(err, repository.ErrAuctionItemUnavailable):
				c.JSON(http.StatusConflict, gin.H{
					"error": "item is unavailable for auction",
				})

			default:
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "failed to create auction listing",
				})
			}
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "auction listing created successfully",
			"data":    listing,
		})
	}
}

// 取得所有拍賣商品
func GetAuctionListingsHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		fmt.Println(">>> HIT GET /auctions")

		listings, err := repository.GetAuctionListings(pool)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": listings,
		})
	}
}

// 取得單一拍賣商品詳情
func GetAuctionListingByIDHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		listingID := c.Param("id")

		listing, err := repository.GetAuctionListingByID(pool, listingID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "auction listing not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": listing,
		})
	}
}

// 對某個拍賣商品出價

func PlaceBidHandler(
	pool *pgxpool.Pool,
	cfg *config.Config,
	auctionHub *stream.AuctionEventHub,
) gin.HandlerFunc {
	return func(c *gin.Context) {

		// 1. 驗證玩家身分
		userID, err := getUserIDFromCookie(c, cfg)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized",
			})
			return
		}

		// 2. 取得拍賣 ID
		listingID := c.Param("id")

		// 3. 解析出價請求
		var req models.PlaceBidRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 4. 執行 Repository 競標交易
		err = repository.PlaceBid(
			pool,
			listingID,
			userID,
			req.BidAmount,
		)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 5. Transaction Commit 成功後才發布事件
		// 不修改資料庫，只通知訂閱該拍賣的前端。
		auctionHub.Publish(stream.AuctionEvent{
			Type:      "auction.bid.updated",
			ListingID: listingID,
			Timestamp: time.Now().UTC(),
		})

		// 6. 回傳出價成功
		c.JSON(http.StatusOK, gin.H{
			"message": "bid placed successfully",
		})
	}
}

// 取得某個拍賣商品的所有出價紀錄
func GetAuctionBidsHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		listingID := c.Param("id")

		bids, err := repository.GetAuctionBidsByListingID(pool, listingID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": bids,
		})
	}
}
