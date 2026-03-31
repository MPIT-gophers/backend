CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- =====================================================
-- USERS
-- Глобальные аккаунты через MAX.
-- Авторизация только через MAX, пароля нет.
-- =====================================================
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    max_user_id VARCHAR(255) NOT NULL UNIQUE,
    full_name VARCHAR(255) NOT NULL,
    phone VARCHAR(32) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE users IS 'Глобальные аккаунты через MAX';
COMMENT ON COLUMN users.max_user_id IS 'Внешний уникальный идентификатор пользователя в MAX';

-- =====================================================
-- EVENTS
-- Событие создаётся при старте генерации.
-- Здесь лежат входные параметры генерации и итоговый статус.
-- =====================================================
CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    city VARCHAR(255) NOT NULL,
    event_date DATE NOT NULL,
    event_time TIME NOT NULL,
    expected_guest_count INTEGER NOT NULL CHECK (expected_guest_count > 0),
    budget NUMERIC(12,2) NOT NULL CHECK (budget >= 0),

    title VARCHAR(255),
    description TEXT,

    status VARCHAR(32) NOT NULL DEFAULT 'generating'
        CHECK (status IN ('draft', 'generating', 'ready', 'failed', 'cancelled')),

    generation_started_at TIMESTAMPTZ,
    generation_finished_at TIMESTAMPTZ,
    generation_error TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE events IS 'События и статусы генерации';
COMMENT ON COLUMN events.status IS 'draft/generating/ready/failed/cancelled';

-- =====================================================
-- EVENT_USERS
-- Роли пользователей внутри конкретного события.
-- =====================================================
CREATE TABLE IF NOT EXISTS event_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    role VARCHAR(32) NOT NULL
        CHECK (role IN ('organizer', 'co_host')),

    status VARCHAR(32) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'removed')),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (event_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_event_users_event_id ON event_users(event_id);
CREATE INDEX IF NOT EXISTS idx_event_users_user_id ON event_users(user_id);

COMMENT ON TABLE event_users IS 'Организаторы/со-организаторы внутри события';
COMMENT ON COLUMN event_users.role IS 'organizer или co_host';

-- =====================================================
-- EVENT_INVITES
-- Многоразовые deep-link ссылки на событие.
-- =====================================================
CREATE TABLE IF NOT EXISTS event_invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    created_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    token VARCHAR(255) NOT NULL UNIQUE,
    title VARCHAR(255),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    expires_at TIMESTAMPTZ,
    usage_count INTEGER NOT NULL DEFAULT 0 CHECK (usage_count >= 0),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_event_invites_event_id ON event_invites(event_id);

COMMENT ON TABLE event_invites IS 'Многоразовые deep-link ссылки на событие';

-- =====================================================
-- EVENT_GUESTS
-- RSVP-гости события и их статусы.
-- =====================================================
CREATE TABLE IF NOT EXISTS event_guests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    invite_id UUID REFERENCES event_invites(id) ON DELETE SET NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,

    full_name VARCHAR(255) NOT NULL,
    phone VARCHAR(32),

    approval_status VARCHAR(32) NOT NULL DEFAULT 'pending'
        CHECK (approval_status IN ('pending', 'approved', 'rejected')),

    attendance_status VARCHAR(32) NOT NULL DEFAULT 'pending'
        CHECK (attendance_status IN ('pending', 'confirmed', 'declined')),

    comment TEXT,
    plus_one_count INTEGER NOT NULL DEFAULT 0 CHECK (plus_one_count >= 0),

    approved_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    approved_at TIMESTAMPTZ,
    responded_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_event_guests_event_id ON event_guests(event_id);
CREATE INDEX IF NOT EXISTS idx_event_guests_user_id ON event_guests(user_id);
CREATE INDEX IF NOT EXISTS idx_event_guests_invite_id ON event_guests(invite_id);
CREATE INDEX IF NOT EXISTS idx_event_guests_event_approval ON event_guests(event_id, approval_status);
CREATE INDEX IF NOT EXISTS idx_event_guests_event_attendance ON event_guests(event_id, attendance_status);

CREATE UNIQUE INDEX IF NOT EXISTS uq_event_guests_event_user_not_null
    ON event_guests(event_id, user_id)
    WHERE user_id IS NOT NULL;

COMMENT ON TABLE event_guests IS 'RSVP-гости события и их статусы';
COMMENT ON COLUMN event_guests.approval_status IS 'Решение организатора: pending/approved/rejected';
COMMENT ON COLUMN event_guests.attendance_status IS 'Ответ гостя: pending/confirmed/declined';

-- =====================================================
-- EVENT_LOCATIONS
-- Карточки локаций от n8n/GigaChat.
-- =====================================================
CREATE TABLE IF NOT EXISTS event_locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,

    title VARCHAR(255) NOT NULL,
    address TEXT,
    contacts TEXT,
    ai_comment TEXT,
    ai_score NUMERIC(5,2),
    sort_order INTEGER NOT NULL DEFAULT 0,

    source VARCHAR(32) NOT NULL DEFAULT 'initial'
        CHECK (source IN ('initial', 'regenerated')),

    is_rejected BOOLEAN NOT NULL DEFAULT FALSE,
    rejected_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_event_locations_event_id ON event_locations(event_id);
CREATE INDEX IF NOT EXISTS idx_event_locations_event_sort ON event_locations(event_id, sort_order);

COMMENT ON TABLE event_locations IS 'Карточки локаций от n8n/GigaChat';
COMMENT ON COLUMN event_locations.source IS 'initial = первая генерация, regenerated = замена после перегенерации';

-- =====================================================
-- Триггер на updated_at для всех таблиц, где есть это поле.
-- =====================================================
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_users_set_updated_at ON users;
CREATE TRIGGER trg_users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_events_set_updated_at ON events;
CREATE TRIGGER trg_events_set_updated_at
BEFORE UPDATE ON events
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_event_users_set_updated_at ON event_users;
CREATE TRIGGER trg_event_users_set_updated_at
BEFORE UPDATE ON event_users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_event_invites_set_updated_at ON event_invites;
CREATE TRIGGER trg_event_invites_set_updated_at
BEFORE UPDATE ON event_invites
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_event_guests_set_updated_at ON event_guests;
CREATE TRIGGER trg_event_guests_set_updated_at
BEFORE UPDATE ON event_guests
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_event_locations_set_updated_at ON event_locations;
CREATE TRIGGER trg_event_locations_set_updated_at
BEFORE UPDATE ON event_locations
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
