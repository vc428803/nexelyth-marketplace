package stream

import (
	"sync"
	"time"
)

// AuctionEvent 代表一次拍賣狀態變更通知。
type AuctionEvent struct {
	Type      string    `json:"type"`
	ListingID string    `json:"listing_id"`
	Timestamp time.Time `json:"timestamp"`
}

// AuctionEventHub 管理各筆拍賣的 SSE 訂閱者。
type AuctionEventHub struct {
	// 不同使用者可能同時訂閱或離開，Go 的 map 不能在沒有同步保護的情況下被並行修改。
	mu sync.RWMutex

	// listingID -> 訂閱者集合
	subscribers map[string]map[chan AuctionEvent]struct{}
}

func NewAuctionEventHub() *AuctionEventHub {
	return &AuctionEventHub{
		subscribers: make(map[string]map[chan AuctionEvent]struct{}),
	}
}

// Subscribe 訂閱指定拍賣。
func (h *AuctionEventHub) Subscribe(
	listingID string,
) chan AuctionEvent {
	// 使用 Buffered Channel。 每位訂閱者有 16 筆事件的暫存空間。慢速用戶端不會阻塞整個競標流程。
	ch := make(chan AuctionEvent, 16)

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.subscribers[listingID] == nil {
		h.subscribers[listingID] =
			make(map[chan AuctionEvent]struct{})
	}

	h.subscribers[listingID][ch] = struct{}{}

	return ch
}

// Unsubscribe 取消訂閱。
func (h *AuctionEventHub) Unsubscribe(
	listingID string,
	ch chan AuctionEvent,
) {
	h.mu.Lock()
	defer h.mu.Unlock()

	subs := h.subscribers[listingID]
	if subs == nil {
		return
	}

	if _, exists := subs[ch]; !exists {
		return
	}

	delete(subs, ch)
	close(ch)

	if len(subs) == 0 {
		delete(h.subscribers, listingID)
	}
}

// Publish 將事件發送給指定拍賣的所有訂閱者。
func (h *AuctionEventHub) Publish(
	event AuctionEvent,
) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	subs := h.subscribers[event.ListingID]

	for ch := range subs {
		select {
		case ch <- event:
			// 成功放入訂閱者佇列

		default:
			// 不讓速度較慢的用戶端阻塞其他訂閱者。
			// 此事件可能遺失，前端重連時需要重新同步。
		}
	}
}
