-- 000004_wishlist_funds.up.sql

ALTER TABLE wishlist_items ADD COLUMN current_fund NUMERIC(12,2) NOT NULL DEFAULT 0;
