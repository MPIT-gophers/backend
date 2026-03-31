CREATE TABLE IF NOT EXISTS auth_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(32) NOT NULL
        CHECK (provider IN ('max', 'telegram')),
    status VARCHAR(32) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'completed', 'exchanged', 'expired', 'failed')),
    provider_user_id VARCHAR(255),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    error_code VARCHAR(64),
    expires_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    exchanged_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_status_expires_at
    ON auth_sessions(status, expires_at);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_user_id
    ON auth_sessions(user_id);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_provider_user_id
    ON auth_sessions(provider, provider_user_id);

COMMENT ON TABLE auth_sessions IS 'Одноразовые auth-сессии для mobile login flow через внешних провайдеров';
COMMENT ON COLUMN auth_sessions.status IS 'pending/completed/exchanged/expired/failed';
COMMENT ON COLUMN auth_sessions.provider_user_id IS 'Внешний идентификатор пользователя у провайдера';

DROP TRIGGER IF EXISTS trg_auth_sessions_set_updated_at ON auth_sessions;
CREATE TRIGGER trg_auth_sessions_set_updated_at
BEFORE UPDATE ON auth_sessions
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
