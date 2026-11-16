package stream

import (
	"testing"
	"time"
)

func TestAuctionEventHubPublish(t *testing.T) {
	hub := NewAuctionEventHub()

	// 兩個不同的拍賣
	auctionA := "auction-A"
	auctionB := "auction-B"

	// 玩家訂閱不同拍賣
	subscriberA := hub.Subscribe(auctionA)
	subscriberB := hub.Subscribe(auctionB)

	defer hub.Unsubscribe(auctionA, subscriberA)
	defer hub.Unsubscribe(auctionB, subscriberB)

	// 模擬 auction-A 有玩家出價
	event := AuctionEvent{
		Type:      "auction.bid.updated",
		ListingID: auctionA,
		Timestamp: time.Now(),
	}

	hub.Publish(event)

	// auction-A 必須收到事件
	select {
	case received := <-subscriberA:
		if received.Type != event.Type ||
			received.ListingID != auctionA {
			t.Fatalf("unexpected event: %+v", received)
		}

	case <-time.After(time.Second):
		t.Fatal("auction-A did not receive event")
	}

	// auction-B 不應收到 auction-A 的事件
	select {
	case received := <-subscriberB:
		t.Fatalf(
			"auction-B received unrelated event: %+v",
			received,
		)

	default:
		// 正確：沒有收到其他拍賣事件
	}
}
