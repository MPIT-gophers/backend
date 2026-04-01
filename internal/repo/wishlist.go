package repo

import (
	"context"
	"fmt"

	"eventAI/internal/entities/core"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WishlistRepository interface {
	SaveParsedWishlist(ctx context.Context, eventID uuid.UUID, items []core.WishlistItem, antiItems []core.AntiWishlistItem) error
	GetWishlist(ctx context.Context, eventID uuid.UUID) ([]core.WishlistItem, error)
	GetAntiWishlist(ctx context.Context, eventID uuid.UUID) ([]core.AntiWishlistItem, error)
	AddWishlistItem(ctx context.Context, item *core.WishlistItem) error
	BookItem(ctx context.Context, itemID, guestID uuid.UUID) error
	FundItem(ctx context.Context, itemID uuid.UUID, amount float64) (float64, error)
}

type wishlistRepository struct {
	db *pgxpool.Pool
}

func NewWishlistRepository(db *pgxpool.Pool) WishlistRepository {
	return &wishlistRepository{
		db: db,
	}
}

func (r *wishlistRepository) SaveParsedWishlist(ctx context.Context, eventID uuid.UUID, items []core.WishlistItem, antiItems []core.AntiWishlistItem) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()

	// Сохраняем элементы вишлиста
	for _, item := range items {
		_, err := tx.Exec(ctx,
			`INSERT INTO wishlist_items (event_id, name, estimated_price, current_fund) VALUES ($1, $2, $3, $4)`,
			eventID, item.Name, item.EstimatedPrice, item.CurrentFund,
		)
		if err != nil {
			return err
		}
	}

	// Сохраняем элементы анти-вишлиста
	for _, antiItem := range antiItems {
		_, err := tx.Exec(ctx,
			`INSERT INTO anti_wishlist (event_id, stop_word) VALUES ($1, $2)`,
			eventID, antiItem.StopWord,
		)
		if err != nil {
			return err
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (r *wishlistRepository) GetWishlist(ctx context.Context, eventID uuid.UUID) ([]core.WishlistItem, error) {
	rows, err := r.db.Query(ctx, `SELECT id, event_id, name, estimated_price, current_fund, is_booked, booked_by_guest_id, created_at, updated_at FROM wishlist_items WHERE event_id = $1 ORDER BY created_at ASC`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []core.WishlistItem
	for rows.Next() {
		var item core.WishlistItem
		if err := rows.Scan(&item.ID, &item.EventID, &item.Name, &item.EstimatedPrice, &item.CurrentFund, &item.IsBooked, &item.BookedByGuestID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *wishlistRepository) GetAntiWishlist(ctx context.Context, eventID uuid.UUID) ([]core.AntiWishlistItem, error) {
	rows, err := r.db.Query(ctx, `SELECT id, event_id, stop_word, created_at, updated_at FROM anti_wishlist WHERE event_id = $1 ORDER BY created_at ASC`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var antiItems []core.AntiWishlistItem
	for rows.Next() {
		var item core.AntiWishlistItem
		if err := rows.Scan(&item.ID, &item.EventID, &item.StopWord, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		antiItems = append(antiItems, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return antiItems, nil
}

func (r *wishlistRepository) AddWishlistItem(ctx context.Context, item *core.WishlistItem) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO wishlist_items (event_id, name, estimated_price, current_fund, is_booked, booked_by_guest_id) 
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at, updated_at`,
		item.EventID, item.Name, item.EstimatedPrice, item.CurrentFund, item.IsBooked, item.BookedByGuestID,
	).Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt)
	return err
}

func (r *wishlistRepository) BookItem(ctx context.Context, itemID, guestID uuid.UUID) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var isBooked bool
	var currentFund float64
	err = tx.QueryRow(ctx, `SELECT is_booked, current_fund FROM wishlist_items WHERE id = $1 FOR UPDATE`, itemID).Scan(&isBooked, &currentFund)
	if err != nil {
		return err
	}

	if isBooked {
		return fmt.Errorf("item already booked")
	}
	if currentFund > 0 {
		return fmt.Errorf("item already has funds, cannot book entirely")
	}

	_, err = tx.Exec(ctx, `UPDATE wishlist_items SET is_booked = true, booked_by_guest_id = $1 WHERE id = $2`, guestID, itemID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *wishlistRepository) FundItem(ctx context.Context, itemID uuid.UUID, amount float64) (float64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var isBooked bool
	var currentFund float64
	err = tx.QueryRow(ctx, `SELECT is_booked, current_fund FROM wishlist_items WHERE id = $1 FOR UPDATE`, itemID).Scan(&isBooked, &currentFund)
	if err != nil {
		return 0, err
	}

	if isBooked {
		return 0, fmt.Errorf("item is already booked")
	}

	newFund := currentFund + amount
	err = tx.QueryRow(ctx, `UPDATE wishlist_items SET current_fund = $1 WHERE id = $2 RETURNING current_fund`, newFund, itemID).Scan(&newFund)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return newFund, nil
}
