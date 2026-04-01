package service

import (
	"context"
	"fmt"
	"eventAI/internal/entities/core"
	"eventAI/internal/repo"

	"github.com/google/uuid"
)

type WishlistService interface {
	ParseWishlist(ctx context.Context, eventID uuid.UUID, text string) ([]core.WishlistItem, []core.AntiWishlistItem, error)
	GetWishlist(ctx context.Context, eventID uuid.UUID) ([]core.WishlistItem, error)
	GetWishlistForUser(ctx context.Context, userID, eventID uuid.UUID) ([]core.WishlistItem, error)
	SubmitGuestIdea(ctx context.Context, eventID, userID uuid.UUID, ideaText string) (bool, string, *core.WishlistItem, error)
	BookItem(ctx context.Context, eventID, itemID, userID uuid.UUID) error
	FundItem(ctx context.Context, eventID, itemID, userID uuid.UUID, amount float64) (float64, error)
}

type RoleProvider interface {
	GetAccessRole(ctx context.Context, userID string, eventID string) (string, error)
}

type AIParser interface {
	ParseWishlistText(ctx context.Context, text string) ([]core.WishlistItem, []core.AntiWishlistItem, error)
	ValidateIdea(ctx context.Context, idea string, antiItems []core.AntiWishlistItem) (bool, string, error)
}

type wishlistService struct {
	repo         repo.WishlistRepository
	parser       AIParser
	roleProvider RoleProvider
}

func NewWishlistService(r repo.WishlistRepository, p AIParser, rp RoleProvider) WishlistService {
	return &wishlistService{
		repo:         r,
		parser:       p,
		roleProvider: rp,
	}
}

func (s *wishlistService) ParseWishlist(ctx context.Context, eventID uuid.UUID, text string) ([]core.WishlistItem, []core.AntiWishlistItem, error) {
	// Call AI/LLM to parse the free text into items and anti items.
	items, antiItems, err := s.parser.ParseWishlistText(ctx, text)
	if err != nil {
		return nil, nil, err
	}

	err = s.repo.SaveParsedWishlist(ctx, eventID, items, antiItems)
	if err != nil {
		return nil, nil, err
	}

	// Reload the wishlist to return items with inserted IDs
	savedItems, err := s.repo.GetWishlist(ctx, eventID)
	if err != nil {
		return nil, nil, err
	}

	savedAntiItems, err := s.repo.GetAntiWishlist(ctx, eventID)
	if err != nil {
		return nil, nil, err
	}

	return savedItems, savedAntiItems, nil
}

func (s *wishlistService) GetWishlist(ctx context.Context, eventID uuid.UUID) ([]core.WishlistItem, error) {
	return s.repo.GetWishlist(ctx, eventID)
}

func (s *wishlistService) GetWishlistForUser(ctx context.Context, userID, eventID uuid.UUID) ([]core.WishlistItem, error) {
	items, err := s.repo.GetWishlist(ctx, eventID)
	if err != nil {
		return nil, err
	}

	role, err := s.roleProvider.GetAccessRole(ctx, userID.String(), eventID.String())
	if err != nil {
		return nil, err
	}

	// Маскировка: организатор не должен видеть статус блокировки и сборы
	if role == "organizer" {
		for i := range items {
			items[i].IsBooked = false
			items[i].BookedByGuestID = nil
			items[i].CurrentFund = 0
		}
	}

	return items, nil
}

func (s *wishlistService) SubmitGuestIdea(ctx context.Context, eventID, userID uuid.UUID, ideaText string) (bool, string, *core.WishlistItem, error) {
	antiItems, err := s.repo.GetAntiWishlist(ctx, eventID)
	if err != nil {
		return false, "", nil, fmt.Errorf("failed to get anti wishlist: %w", err)
	}

	allowed, reason, err := s.parser.ValidateIdea(ctx, ideaText, antiItems)
	if err != nil {
		return false, "", nil, fmt.Errorf("failed to validate idea: %w", err)
	}

	if !allowed {
		return false, reason, nil, nil
	}

	item := &core.WishlistItem{
		EventID:         eventID,
		Name:            ideaText,
		IsBooked:        true,
		BookedByGuestID: &userID,
	}

	if err := s.repo.AddWishlistItem(ctx, item); err != nil {
		return false, "", nil, fmt.Errorf("failed to save idea: %w", err)
	}

	return true, "", item, nil
}

func (s *wishlistService) BookItem(ctx context.Context, eventID, itemID, userID uuid.UUID) error {
	role, err := s.roleProvider.GetAccessRole(ctx, userID.String(), eventID.String())
	if err != nil {
		return err
	}
	if role == "organizer" {
		return fmt.Errorf("organizer cannot book wishlist items")
	}

	return s.repo.BookItem(ctx, itemID, userID)
}

func (s *wishlistService) FundItem(ctx context.Context, eventID, itemID, userID uuid.UUID, amount float64) (float64, error) {
	if amount <= 0 {
		return 0, fmt.Errorf("amount must be greater than zero")
	}

	role, err := s.roleProvider.GetAccessRole(ctx, userID.String(), eventID.String())
	if err != nil {
		return 0, err
	}
	if role == "organizer" {
		return 0, fmt.Errorf("organizer cannot fund wishlist items")
	}

	return s.repo.FundItem(ctx, itemID, amount)
}
