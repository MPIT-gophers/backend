-- name: GetUserByOAuth :one
SELECT
    u.id,
    u.full_name,
    u.phone,
    u.created_at,
    u.updated_at
FROM user_oauth_accounts ua
INNER JOIN users u ON u.id = ua.user_id
WHERE ua.provider = $1
  AND ua.provider_user_id = $2;

-- name: GetUserByPhone :one
SELECT
    id,
    full_name,
    phone,
    created_at,
    updated_at
FROM users
WHERE phone = sqlc.arg(phone);

-- name: CreateUser :one
INSERT INTO users (
    full_name,
    phone
) VALUES (
    sqlc.arg(full_name),
    sqlc.narg(phone)
)
RETURNING
    id,
    full_name,
    phone,
    created_at,
    updated_at;

-- name: InsertUserOAuthAccount :exec
INSERT INTO user_oauth_accounts (
    user_id,
    provider,
    provider_user_id
) VALUES (
    $1,
    $2,
    $3
);

-- name: UpdateUserProfile :one
UPDATE users
SET
    full_name = CASE WHEN sqlc.arg(full_name_set)::bool THEN sqlc.arg(full_name) ELSE full_name END,
    phone = CASE WHEN sqlc.arg(phone_set)::bool THEN sqlc.narg(phone) ELSE phone END
WHERE id = sqlc.arg(id)
RETURNING
    id,
    full_name,
    phone,
    created_at,
    updated_at;

-- name: EnrichUserFromProvider :one
UPDATE users
SET
    full_name = CASE WHEN sqlc.arg(full_name_set)::bool THEN sqlc.arg(full_name) ELSE full_name END,
    phone = CASE WHEN sqlc.arg(phone_set)::bool THEN sqlc.narg(phone) ELSE phone END
WHERE id = sqlc.arg(id)
RETURNING
    id,
    full_name,
    phone,
    created_at,
    updated_at;

-- name: GetUserProfileByID :one
SELECT
    full_name,
    phone
FROM users
WHERE id = sqlc.arg(id);
