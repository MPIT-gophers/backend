CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- =====================================================
-- USERS
-- Глобальные аккаунты системы.
-- Здесь хранится сама сущность пользователя без привязки к конкретному OAuth-провайдеру.
-- =====================================================
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name VARCHAR(255) NOT NULL,
    phone VARCHAR(32),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_users_phone_not_null
    ON users(phone)
    WHERE phone IS NOT NULL;

COMMENT ON TABLE users IS 'Глобальные аккаунты системы';
COMMENT ON COLUMN users.phone IS 'Телефон пользователя в нормализованном формате, если провайдер его отдал';

-- =====================================================
-- USER_OAUTH_ACCOUNTS
-- Привязки глобального пользователя к OAuth/SSO-провайдерам.
-- Сейчас используем MAX, позже сюда же без перелома схемы добавится Telegram.
-- =====================================================
CREATE TABLE IF NOT EXISTS user_oauth_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    provider VARCHAR(32) NOT NULL
        CHECK (provider IN ('max', 'telegram')),
    provider_user_id VARCHAR(255) NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (provider, provider_user_id),
    UNIQUE (user_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_user_oauth_accounts_user_id ON user_oauth_accounts(user_id);
CREATE INDEX IF NOT EXISTS idx_user_oauth_accounts_provider_user_id
    ON user_oauth_accounts(provider, provider_user_id);

COMMENT ON TABLE user_oauth_accounts IS 'Связки пользователей с внешними OAuth/SSO-провайдерами';
COMMENT ON COLUMN user_oauth_accounts.provider IS 'max сейчас, telegram позже';
COMMENT ON COLUMN user_oauth_accounts.provider_user_id IS 'Внешний идентификатор пользователя у провайдера';

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
    selected_variant_id UUID,

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
-- EVENT_VARIANTS
-- Один вариант = один полный результат генерации от LLM/n8n.
-- Пользователь в итоге выбирает один вариант целиком.
-- Источник истины по финальному выбору хранится в events.selected_variant_id.
-- =====================================================
CREATE TABLE IF NOT EXISTS event_variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,

    variant_number INTEGER NOT NULL CHECK (variant_number > 0),
    title VARCHAR(255),
    description TEXT,

    status VARCHAR(32) NOT NULL DEFAULT 'ready'
        CHECK (status IN ('generating', 'ready', 'failed', 'rejected')),

    llm_request_id VARCHAR(255),
    generation_error TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (event_id, variant_number)
);

CREATE INDEX IF NOT EXISTS idx_event_variants_event_id ON event_variants(event_id);
CREATE INDEX IF NOT EXISTS idx_event_variants_event_status ON event_variants(event_id, status);

COMMENT ON TABLE event_variants IS 'Сгенерированные варианты сценария/подборки для события';
COMMENT ON COLUMN event_variants.variant_number IS 'Порядковый номер варианта внутри события';

-- =====================================================
-- EVENTS.selected_variant_id
-- Финальный вариант, который выбрал пользователь.
-- =====================================================
ALTER TABLE events
    ADD CONSTRAINT fk_events_selected_variant
    FOREIGN KEY (selected_variant_id)
    REFERENCES event_variants(id)
    ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_events_selected_variant_id ON events(selected_variant_id);

COMMENT ON COLUMN events.selected_variant_id IS 'Ссылка на выбранный пользователем вариант события';

-- =====================================================
-- EVENT_LOCATIONS
-- Карточки локаций теперь принадлежат конкретному variant-у.
-- =====================================================
CREATE TABLE IF NOT EXISTS event_locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    variant_id UUID NOT NULL REFERENCES event_variants(id) ON DELETE CASCADE,

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
CREATE INDEX IF NOT EXISTS idx_event_locations_variant_id ON event_locations(variant_id);
CREATE INDEX IF NOT EXISTS idx_event_locations_variant_sort ON event_locations(variant_id, sort_order);

COMMENT ON TABLE event_locations IS 'Карточки локаций внутри конкретного сгенерированного варианта';
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

DROP TRIGGER IF EXISTS trg_user_oauth_accounts_set_updated_at ON user_oauth_accounts;
CREATE TRIGGER trg_user_oauth_accounts_set_updated_at
BEFORE UPDATE ON user_oauth_accounts
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

DROP TRIGGER IF EXISTS trg_event_variants_set_updated_at ON event_variants;
CREATE TRIGGER trg_event_variants_set_updated_at
BEFORE UPDATE ON event_variants
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_event_locations_set_updated_at ON event_locations;
CREATE TRIGGER trg_event_locations_set_updated_at
BEFORE UPDATE ON event_locations
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
