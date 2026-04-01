package ai

import (
	"context"

	"eventAI/internal/entities/core"
)

type MockWishlistParser struct{}

func NewMockWishlistParser() *MockWishlistParser {
	return &MockWishlistParser{}
}

func (p *MockWishlistParser) ParseWishlistText(ctx context.Context, text string) ([]core.WishlistItem, []core.AntiWishlistItem, error) {
	// Демонстрационный мок: всегда возвращает статические данные.
	// В будущем здесь будет реальный вызов LLM (GigaChat).
	price := 50000.0
	items := []core.WishlistItem{
		{
			Name:           "Квадрокоптер",
			EstimatedPrice: &price,
		},
	}

	antiItems := []core.AntiWishlistItem{
		{
			StopWord: "орехи",
		},
	}

	return items, antiItems, nil
}

func (p *MockWishlistParser) ValidateIdea(ctx context.Context, idea string, antiItems []core.AntiWishlistItem) (bool, string, error) {
	panic("not implemented")
}
