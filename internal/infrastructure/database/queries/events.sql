-- name: CreateEvent :one
INSERT INTO events (
    city,
    event_date,
    event_time,
    expected_guest_count,
    budget,
    status
) VALUES (
    sqlc.arg(city),
    sqlc.arg(event_date)::date,
    sqlc.arg(event_time)::time,
    sqlc.arg(expected_guest_count),
    sqlc.arg(budget)::numeric,
    'draft'
)
RETURNING
    id,
    city,
    event_date,
    event_time,
    expected_guest_count,
    budget,
    title,
    description,
    status,
    selected_variant_id,
    created_at,
    updated_at;

-- name: AddEventUser :exec
INSERT INTO event_users (
    event_id,
    user_id,
    role,
    status
) VALUES (
    sqlc.arg(event_id),
    sqlc.arg(user_id),
    'organizer',
    'active'
);

-- name: CreateEventInvite :exec
INSERT INTO event_invites (
    event_id,
    created_by_user_id,
    token,
    title
) VALUES (
    sqlc.arg(event_id),
    sqlc.arg(created_by_user_id),
    sqlc.arg(token),
    'primary'
);

-- name: ListMyEvents :many
WITH related AS (
    SELECT
        eu.event_id,
        eu.role AS access_role,
        NULL::text AS approval_status,
        NULL::text AS attendance_status,
        1 AS priority
    FROM event_users eu
    WHERE eu.user_id = sqlc.arg(user_id)
      AND eu.status = 'active'

    UNION ALL

    SELECT
        eg.event_id,
        CASE
            WHEN eg.approval_status <> 'approved' THEN 'guest_' || eg.approval_status
            WHEN eg.attendance_status = 'confirmed' THEN 'guest_confirmed'
            WHEN eg.attendance_status = 'declined' THEN 'guest_declined'
            ELSE 'guest_approved'
        END AS access_role,
        eg.approval_status::text,
        eg.attendance_status::text,
        2 AS priority
    FROM event_guests eg
    WHERE eg.user_id = sqlc.arg(user_id)
)
SELECT DISTINCT ON (e.id)
    e.id,
    e.city,
    e.event_date,
    e.event_time,
    e.expected_guest_count,
    e.budget,
    e.title,
    e.description,
    e.status,
    e.selected_variant_id,
    related.access_role,
    related.approval_status,
    related.attendance_status,
    e.created_at,
    e.updated_at
FROM events e
INNER JOIN related ON related.event_id = e.id
ORDER BY e.id, related.priority, e.created_at DESC;

-- name: GetInviteByToken :one
SELECT
    id,
    event_id
FROM event_invites
WHERE token = sqlc.arg(token)
  AND is_active = TRUE
  AND (expires_at IS NULL OR expires_at > NOW());

-- name: UpsertEventGuestByUser :exec
INSERT INTO event_guests (
    event_id,
    invite_id,
    user_id,
    full_name,
    phone
) VALUES (
    sqlc.arg(event_id),
    sqlc.arg(invite_id),
    sqlc.arg(user_id),
    sqlc.arg(full_name),
    sqlc.narg(phone)
)
ON CONFLICT (event_id, user_id) WHERE user_id IS NOT NULL
DO UPDATE SET
    invite_id = EXCLUDED.invite_id,
    full_name = EXCLUDED.full_name,
    phone = EXCLUDED.phone,
    updated_at = NOW();

-- name: IncrementInviteUsage :exec
UPDATE event_invites
SET usage_count = usage_count + 1
WHERE id = sqlc.arg(id);

-- name: GetEventWithGuestAccess :one
SELECT
    e.id,
    e.city,
    e.event_date,
    e.event_time,
    e.expected_guest_count,
    e.budget,
    e.title,
    e.description,
    e.status,
    e.selected_variant_id,
    CASE
        WHEN eg.approval_status <> 'approved' THEN 'guest_' || eg.approval_status
        WHEN eg.attendance_status = 'confirmed' THEN 'guest_confirmed'
        WHEN eg.attendance_status = 'declined' THEN 'guest_declined'
        ELSE 'guest_approved'
    END AS access_role,
    eg.approval_status::text,
    eg.attendance_status::text,
    e.created_at,
    e.updated_at
FROM events e
INNER JOIN event_guests eg ON eg.event_id = e.id AND eg.user_id = sqlc.arg(user_id)
WHERE e.id = sqlc.arg(event_id);

-- name: GetEventByID :one
SELECT
    id,
    city,
    event_date,
    event_time,
    expected_guest_count,
    budget,
    title,
    description,
    status,
    selected_variant_id,
    created_at,
    updated_at
FROM events
WHERE id = sqlc.arg(id);

-- name: GetEventAccessRole :one
WITH related AS (
    SELECT eu.role AS access_role, 1 AS priority
    FROM event_users eu
    WHERE eu.event_id = sqlc.arg(event_id)
      AND eu.user_id = sqlc.arg(user_id)
      AND eu.status = 'active'

    UNION ALL

    SELECT
        CASE
            WHEN eg.approval_status <> 'approved' THEN 'guest_' || eg.approval_status
            WHEN eg.attendance_status = 'confirmed' THEN 'guest_confirmed'
            WHEN eg.attendance_status = 'declined' THEN 'guest_declined'
            ELSE 'guest_approved'
        END AS access_role,
        2 AS priority
    FROM event_guests eg
    WHERE eg.event_id = sqlc.arg(event_id)
      AND eg.user_id = sqlc.arg(user_id)
)
SELECT access_role
FROM related
ORDER BY priority
LIMIT 1;

-- name: ListEventVariantsByEventID :many
SELECT
    id,
    event_id,
    variant_number,
    title,
    description,
    status,
    llm_request_id,
    generation_error,
    created_at,
    updated_at
FROM event_variants
WHERE event_id = sqlc.arg(event_id)
ORDER BY variant_number;

-- name: GetEventInviteByEventID :one
SELECT token
FROM event_invites
WHERE event_id = sqlc.arg(event_id)
  AND is_active = TRUE
  AND (expires_at IS NULL OR expires_at > NOW())
LIMIT 1;

-- name: ListEventGuests :many
SELECT
    id,
    event_id,
    user_id,
    full_name,
    phone,
    approval_status,
    attendance_status,
    plus_one_count,
    created_at
FROM event_guests
WHERE event_id = sqlc.arg(event_id)
  AND (sqlc.narg(approval_status)::text IS NULL OR approval_status = sqlc.narg(approval_status)::text)
ORDER BY created_at ASC;

-- name: UpdateGuestApprovalStatus :one
UPDATE event_guests
SET
    approval_status     = sqlc.arg(approval_status),
    approved_by_user_id = sqlc.arg(approved_by_user_id),
    approved_at         = NOW(),
    updated_at          = NOW()
WHERE id = sqlc.arg(id)
  AND event_id = sqlc.arg(event_id)
RETURNING
    id,
    event_id,
    user_id,
    full_name,
    phone,
    approval_status,
    attendance_status,
    plus_one_count,
    created_at;

-- name: UpdateGuestAttendanceStatus :one
UPDATE event_guests
SET
    attendance_status = sqlc.arg(attendance_status),
    responded_at      = NOW(),
    updated_at        = NOW()
WHERE id = sqlc.arg(id)
  AND event_id = sqlc.arg(event_id)
  AND approval_status = 'approved'
RETURNING
    id,
    event_id,
    user_id,
    full_name,
    phone,
    approval_status,
    attendance_status,
    plus_one_count,
    created_at;

-- name: GetEventGuestStats :one
SELECT
    COUNT(*) FILTER (WHERE approval_status = 'pending')                                    AS pending_approval,
    COUNT(*) FILTER (WHERE approval_status = 'approved')                                   AS approved,
    COUNT(*) FILTER (WHERE approval_status = 'rejected')                                   AS rejected,
    COUNT(*) FILTER (WHERE approval_status = 'approved' AND attendance_status = 'pending') AS attendance_pending,
    COUNT(*) FILTER (WHERE attendance_status = 'confirmed')                                AS confirmed,
    COUNT(*) FILTER (WHERE attendance_status = 'declined')                                 AS declined
FROM event_guests
WHERE event_id = sqlc.arg(event_id);

-- name: ListEventLocationsByEventID :many
SELECT
    id,
    event_id,
    variant_id,
    title,
    address,
    contacts,
    ai_comment,
    ai_score,
    sort_order,
    source,
    is_rejected,
    rejected_at,
    created_at,
    updated_at
FROM event_locations
WHERE event_id = sqlc.arg(event_id)
ORDER BY variant_id, sort_order, created_at;
