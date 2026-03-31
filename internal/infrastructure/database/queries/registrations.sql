-- name: CreateRegistration :one
INSERT INTO registrations (
    full_name,
    email
) VALUES (
    $1,
    $2
)
RETURNING id, full_name, email, created_at;

-- name: ListRegistrations :many
SELECT id, full_name, email, created_at
FROM registrations
ORDER BY id DESC;

