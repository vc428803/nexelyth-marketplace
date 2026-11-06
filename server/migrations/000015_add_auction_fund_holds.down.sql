
DROP TABLE IF EXISTS auction_fund_holds;

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
        'ADJUSTMENT'
    )
);
