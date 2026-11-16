package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"todo_api/internal/stream"

	"github.com/gin-gonic/gin"
)

// AuctionSseHandler 提供指定拍賣的即時事件串流。
func AuctionSseHandler(
	hub *stream.AuctionEventHub,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		listingID := c.Param("id")

		// 1. 設定 SSE Headers
		w := c.Writer

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		// 2. 訂閱指定拍賣
		// 讓 Subscriber 的生命週期跟 SSE 連線綁定，避免 Memory Leak 與 Zombie Channel。
		events := hub.Subscribe(listingID)
		// 如果沒有這個
		// Hub 仍然持有這個 channel
		// GC 無法回收
		// Subscriber 數量越來越多
		// 發送事件時仍然會對已失效 channel 廣播
		// 最後造成 Memory Leak
		defer hub.Unsubscribe(listingID, events)

		// 3. 建立 Flush 控制器
		rc := http.NewResponseController(w)

		// 4. 每 25 秒發送 Ping，維持連線
		pingTicker := time.NewTicker(25 * time.Second)
		defer pingTicker.Stop()

		// 5. 告知前端連線已建立
		if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
			return
		}
		if err := rc.Flush(); err != nil {
			return
		}

		// 6. 等待拍賣事件
		for {
			select {
			// 當 SSE Client 斷線時，Handler 雖然退出了
			case <-c.Request.Context().Done():
				log.Println("Auction SSE client disconnected")
				return

			case event, ok := <-events:
				if !ok {
					return
				}

				payload, err := json.Marshal(event)
				if err != nil {
					log.Printf("Auction SSE marshal error: %v", err)
					continue
				}

				// SSE 標準格式
				if _, err := fmt.Fprintf(
					w,
					"event: %s\ndata: %s\n\n",
					event.Type,
					payload,
				); err != nil {
					return
				}

				if err := rc.Flush(); err != nil {
					return
				}

			case <-pingTicker.C:
				if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
					return
				}
				if err := rc.Flush(); err != nil {
					return
				}
			}
		}
	}
}
