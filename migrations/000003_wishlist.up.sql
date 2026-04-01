-- 000003_wishlist.up.sql

CREATE TABLE IF NOT EXISTS wishlist_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    
    name VARCHAR(255) NOT NULL,
    estimated_price NUMERIC(12,2),
    
    is_booked BOOLEAN NOT NULL DEFAULT FALSE,
    booked_by_guest_id UUID REFERENCES event_guests(id) ON DELETE SET NULL,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wishlist_items_event_id ON wishlist_items(event_id);

COMMENT ON TABLE wishlist_items IS 'Предметы из вишлиста события';

-- Таблица для стоп-слов/анти-подарков
CREATE TABLE IF NOT EXISTS anti_wishlist (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    
    stop_word VARCHAR(255) NOT NULL,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_anti_wishlist_event_id ON anti_wishlist(event_id);

COMMENT ON TABLE anti_wishlist IS 'Стоп-слова для подарков (анти-вишлист)';

-- Триггеры для updated_at
DROP TRIGGER IF EXISTS trg_wishlist_items_set_updated_at ON wishlist_items;
CREATE TRIGGER trg_wishlist_items_set_updated_at
BEFORE UPDATE ON wishlist_items
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_anti_wishlist_set_updated_at ON anti_wishlist;
CREATE TRIGGER trg_anti_wishlist_set_updated_at
BEFORE UPDATE ON anti_wishlist
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
