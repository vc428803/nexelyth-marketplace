
-- 競標資金保留紀錄
CREATE TABLE auction_fund_holds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    listing_id UUID NOT NULL
        REFERENCES auction_listings(id) ON DELETE RESTRICT,

    bid_id UUID NOT NULL UNIQUE
        REFERENCES auction_bids(id) ON DELETE RESTRICT,

    bidder_id UUID NOT NULL
        REFERENCES users(id) ON DELETE RESTRICT,

    amount BIGINT NOT NULL
        CHECK (amount > 0),

    status VARCHAR(20) NOT NULL DEFAULT 'HELD'
        CHECK (status IN ('HELD', 'RELEASED', 'SETTLED')),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 每筆拍賣最多只能有一筆進行中的資金保留
CREATE UNIQUE INDEX uq_auction_fund_holds_active
ON auction_fund_holds(listing_id)
WHERE status = 'HELD';

CREATE INDEX idx_auction_fund_holds_bidder
ON auction_fund_holds(bidder_id);

-- 擴充錢包交易類型
ALTER TABLE wallet_transactions
DROP CONSTRAINT wallet_transactions_transaction_type_check;

ALTER TABLE wallet_transactions
ADD CONSTRAINT wallet_transactions_transaction_type_check
CHECK (
    transaction_type IN (
        'INITIAL_GRANT',
        'PURCHASE',
        'SALE',
        'REWARD',
        'ADJUSTMENT',
        'BID_HOLD',
        'BID_RELEASE'
    )
);
