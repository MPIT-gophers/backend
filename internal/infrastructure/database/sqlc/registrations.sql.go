package sqlc

import "context"

const createRegistration = `-- name: CreateRegistration :one
INSERT INTO registrations (
    full_name,
    email
) VALUES (
    $1,
    $2
)
RETURNING id, full_name, email, created_at
`

type CreateRegistrationParams struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
}

func (q *Queries) CreateRegistration(ctx context.Context, arg CreateRegistrationParams) (Registration, error) {
	row := q.db.QueryRow(ctx, createRegistration, arg.FullName, arg.Email)
	var item Registration
	err := row.Scan(
		&item.ID,
		&item.FullName,
		&item.Email,
		&item.CreatedAt,
	)
	return item, err
}

const listRegistrations = `-- name: ListRegistrations :many
SELECT id, full_name, email, created_at
FROM registrations
ORDER BY id DESC
`

func (q *Queries) ListRegistrations(ctx context.Context) ([]Registration, error) {
	rows, err := q.db.Query(ctx, listRegistrations)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Registration
	for rows.Next() {
		var item Registration
		if err := rows.Scan(
			&item.ID,
			&item.FullName,
			&item.Email,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}
