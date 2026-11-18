package worker

import (
	"testing"
	"time"

	"todo_api/internal/stream"
)

func TestPublishAuctionSettlementEvent(t *testing.T) {
	tests := []struct {
		name          string
		status        string
		expectedEvent string
		shouldPublish bool
	}{
		{
			name:          "Completed auction",
			status:        "completed",
			expectedEvent: "auction.completed",
			shouldPublish: true,
		},
		{
			name:          "Expired auction",
			status:        "expired",
			expectedEvent: "auction.expired",
			shouldPublish: true,
		},
		{
			name:          "Active auction",
			status:        "active",
			expectedEvent: "",
			shouldPublish: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub := stream.NewAuctionEventHub()
			listingID := "test-auction"

			subscriber := hub.Subscribe(listingID)
			defer hub.Unsubscribe(listingID, subscriber)

			published := publishAuctionSettlementEvent(
				hub,
				listingID,
				tt.status,
			)

			if published != tt.shouldPublish {
				t.Fatalf(
					"published = %v, want %v",
					published,
					tt.shouldPublish,
				)
			}

			select {
			case event := <-subscriber:
				if !tt.shouldPublish {
					t.Fatalf(
						"unexpected event: %+v",
						event,
					)
				}

				if event.Type != tt.expectedEvent {
					t.Fatalf(
						"event type = %s, want %s",
						event.Type,
						tt.expectedEvent,
					)
				}

				if event.ListingID != listingID {
					t.Fatalf(
						"listing ID = %s, want %s",
						event.ListingID,
						listingID,
					)
				}

				if event.Timestamp.IsZero() {
					t.Fatal("event timestamp is missing")
				}

			case <-time.After(100 * time.Millisecond):
				if tt.shouldPublish {
					t.Fatal("expected event was not received")
				}
			}
		})
	}
}
