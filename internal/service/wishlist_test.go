package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"eventAI/internal/entities/core"

	"github.com/google/uuid"
)

type stubWishlistRepository struct {
	saveErr      error
	getErr       error
	getAntiErr   error
	savedItems   []core.WishlistItem
	savedAnti    []core.AntiWishlistItem
	returnedItem []core.WishlistItem
	returnedAnti []core.AntiWishlistItem
}

func (s *stubWishlistRepository) SaveParsedWishlist(ctx context.Context, eventID uuid.UUID, items []core.WishlistItem, antiItems []core.AntiWishlistItem) error {
	s.savedItems = items
	s.savedAnti = antiItems
	return s.saveErr
}

func (s *stubWishlistRepository) GetWishlist(ctx context.Context, eventID uuid.UUID) ([]core.WishlistItem, error) {
	return s.returnedItem, s.getErr
}

func (s *stubWishlistRepository) GetAntiWishlist(ctx context.Context, eventID uuid.UUID) ([]core.AntiWishlistItem, error) {
	return s.returnedAnti, s.getAntiErr
}

func (s *stubWishlistRepository) AddWishlistItem(ctx context.Context, item *core.WishlistItem) error {
	s.savedItems = append(s.savedItems, *item)
	return s.saveErr
}

func (s *stubWishlistRepository) BookItem(ctx context.Context, itemID, guestID uuid.UUID) error {
	return s.saveErr
}

func (s *stubWishlistRepository) FundItem(ctx context.Context, itemID uuid.UUID, amount float64) (float64, error) {
	return amount, s.saveErr
}


type stubAIParser struct {
	items     []core.WishlistItem
	antiItems []core.AntiWishlistItem
	err       error
}

func (p *stubAIParser) ParseWishlistText(ctx context.Context, text string) ([]core.WishlistItem, []core.AntiWishlistItem, error) {
	return p.items, p.antiItems, p.err
}

func (p *stubAIParser) ValidateIdea(ctx context.Context, idea string, antiItems []core.AntiWishlistItem) (bool, string, error) {
	// Simple mock behavior: if idea equals "bad", return false
	if idea == "bad" {
		return false, "not allowed", p.err
	}
	return true, "", p.err
}

type stubRoleProvider struct {
	role string
	err  error
}

func (p *stubRoleProvider) GetAccessRole(ctx context.Context, userID string, eventID string) (string, error) {
	return p.role, p.err
}

func TestWishlistService_ParseWishlist(t *testing.T) {
	t.Parallel()

	validEventID := uuid.New()
	price := 100.0

	tests := []struct {
		name              string
		text              string
		parserItems       []core.WishlistItem
		parserAntiItems   []core.AntiWishlistItem
		parserErr         error
		repoSaveErr       error
		repoGetItems      []core.WishlistItem
		repoGetAntiItems  []core.AntiWishlistItem
		expectedItems     []core.WishlistItem
		expectedAntiItems []core.AntiWishlistItem
		wantErr           bool
	}{
		{
			name: "Success",
			text: "test text",
			parserItems: []core.WishlistItem{
				{Name: "Gift", EstimatedPrice: &price},
			},
			parserAntiItems: []core.AntiWishlistItem{
				{StopWord: "bad"},
			},
			parserErr:   nil,
			repoSaveErr: nil,
			repoGetItems: []core.WishlistItem{
				{ID: uuid.New(), Name: "Gift", EstimatedPrice: &price},
			},
			repoGetAntiItems: []core.AntiWishlistItem{
				{ID: uuid.New(), StopWord: "bad"},
			},
			expectedItems: []core.WishlistItem{
				{Name: "Gift", EstimatedPrice: &price}, // IDs mapped below
			},
			expectedAntiItems: []core.AntiWishlistItem{
				{StopWord: "bad"},
			},
			wantErr: false,
		},
		{
			name:            "Parser Error",
			text:            "parse me",
			parserErr:       errors.New("ai offline"),
			repoSaveErr:     nil,
			wantErr:         true,
		},
		{
			name:            "Repository Save Error",
			text:            "save me",
			parserItems:     []core.WishlistItem{{Name: "Item"}},
			parserAntiItems: []core.AntiWishlistItem{},
			parserErr:       nil,
			repoSaveErr:     errors.New("db error"),
			wantErr:         true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repoStub := &stubWishlistRepository{
				saveErr:      tc.repoSaveErr,
				returnedItem: tc.repoGetItems,
				returnedAnti: tc.repoGetAntiItems,
			}
			parserStub := &stubAIParser{
				items:     tc.parserItems,
				antiItems: tc.parserAntiItems,
				err:       tc.parserErr,
			}
			roleStub := &stubRoleProvider{
				role: "guest",
				err:  nil,
			}
			
			svc := NewWishlistService(repoStub, parserStub, roleStub)

			items, antiItems, err := svc.ParseWishlist(context.Background(), validEventID, tc.text)

			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseWishlist() error = %v, wantErr %v", err, tc.wantErr)
			}

			if !tc.wantErr {
				if len(items) != len(tc.expectedItems) {
					t.Fatalf("expected items %d got %d", len(tc.expectedItems), len(items))
				}
				if len(antiItems) != len(tc.expectedAntiItems) {
					t.Fatalf("expected anti items %d got %d", len(tc.expectedAntiItems), len(antiItems))
				}
				
				// Ensure repo was called with parser output
				if !reflect.DeepEqual(repoStub.savedItems, tc.parserItems) {
					t.Errorf("expected saved items to be %v got %v", tc.parserItems, repoStub.savedItems)
				}
			}
		})
	}
}

func TestWishlistService_GetWishlistForUser(t *testing.T) {
	t.Parallel()

	validEventID := uuid.New()
	validUserID := uuid.New()
	price := 1000.0
	guestID := uuid.New()

	baseItem := core.WishlistItem{
		ID:              uuid.New(),
		EventID:         validEventID,
		Name:            "Торт",
		EstimatedPrice:  &price,
		CurrentFund:     500.0,
		IsBooked:        true,
		BookedByGuestID: &guestID,
	}

	tests := []struct {
		name        string
		role        string
		roleErr     error
		repoErr     error
		repoItems   []core.WishlistItem
		wantErr     bool
		checkFields func(*testing.T, []core.WishlistItem)
	}{
		{
			name:      "Organizer gets masked items",
			role:      "organizer",
			repoItems: []core.WishlistItem{baseItem},
			wantErr:   false,
			checkFields: func(t *testing.T, items []core.WishlistItem) {
				for _, item := range items {
					if item.IsBooked {
						t.Errorf("expected IsBooked to be false for organizer")
					}
					if item.BookedByGuestID != nil {
						t.Errorf("expected BookedByGuestID to be nil for organizer")
					}
					if item.CurrentFund != 0 {
						t.Errorf("expected CurrentFund to be 0 for organizer, got %f", item.CurrentFund)
					}
				}
			},
		},
		{
			name:      "Guest gets full items",
			role:      "guest",
			repoItems: []core.WishlistItem{baseItem},
			wantErr:   false,
			checkFields: func(t *testing.T, items []core.WishlistItem) {
				for _, item := range items {
					if !item.IsBooked {
						t.Errorf("expected IsBooked to be true for guest")
					}
					if item.BookedByGuestID == nil {
						t.Errorf("expected BookedByGuestID to not be nil for guest")
					}
					if item.CurrentFund != 500.0 {
						t.Errorf("expected CurrentFund to be 500.0 for guest, got %f", item.CurrentFund)
					}
				}
			},
		},
		{
			name:      "Role error returns error",
			roleErr:   errors.New("role not found"),
			repoItems: []core.WishlistItem{baseItem},
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repoStub := &stubWishlistRepository{
				returnedItem: tc.repoItems,
				getErr:       tc.repoErr,
			}
			parserStub := &stubAIParser{}
			roleStub := &stubRoleProvider{
				role: tc.role,
				err:  tc.roleErr,
			}

			svc := NewWishlistService(repoStub, parserStub, roleStub)

			items, err := svc.GetWishlistForUser(context.Background(), validUserID, validEventID)

			if (err != nil) != tc.wantErr {
				t.Fatalf("GetWishlistForUser() error = %v, wantErr %v", err, tc.wantErr)
			}

			if !tc.wantErr && tc.checkFields != nil {
				tc.checkFields(t, items)
			}
		})
	}
}

func TestWishlistService_SubmitGuestIdea(t *testing.T) {
	t.Parallel()

	validEventID := uuid.New()
	validUserID := uuid.New()

	tests := []struct {
		name         string
		idea         string
		repoGetErr   error
		repoSaveErr  error
		parserErr    error
		antiItems    []core.AntiWishlistItem
		wantAllowed  bool
		wantReason   string
		wantErr      bool
	}{
		{
			name:        "Allowed Idea",
			idea:        "good",
			antiItems:   []core.AntiWishlistItem{},
			wantAllowed: true,
			wantReason:  "",
			wantErr:     false,
		},
		{
			name:        "Blocked Idea",
			idea:        "bad",
			antiItems:   []core.AntiWishlistItem{{StopWord: "bad"}},
			wantAllowed: false,
			wantReason:  "not allowed",
			wantErr:     false,
		},
		{
			name:        "Repo Get Error",
			idea:        "good",
			repoGetErr:  errors.New("db err"),
			wantErr:     true,
		},
		{
			name:        "Repo Save Error",
			idea:        "good",
			repoSaveErr: errors.New("db err"),
			wantErr:     true,
		},
		{
			name:        "Parser Error",
			idea:        "bad",
			parserErr:   errors.New("ai err"),
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repoStub := &stubWishlistRepository{
				returnedAnti: tc.antiItems,
				getAntiErr:   tc.repoGetErr,
				saveErr:      tc.repoSaveErr,
			}
			parserStub := &stubAIParser{err: tc.parserErr}
			roleStub := &stubRoleProvider{}

			svc := NewWishlistService(repoStub, parserStub, roleStub)

			allowed, reason, item, err := svc.SubmitGuestIdea(context.Background(), validEventID, validUserID, tc.idea)

			if (err != nil) != tc.wantErr {
				t.Fatalf("SubmitGuestIdea() error = %v, wantErr %v", err, tc.wantErr)
			}

			if !tc.wantErr {
				if allowed != tc.wantAllowed {
					t.Errorf("expected allowed %v got %v", tc.wantAllowed, allowed)
				}
				if reason != tc.wantReason {
					t.Errorf("expected reason %v got %v", tc.wantReason, reason)
				}
				if allowed && item == nil {
					t.Errorf("expected item to be returned if allowed")
				}
				if !allowed && item != nil {
					t.Errorf("expected no item to be returned if not allowed")
				}
			}
		})
	}
}

func TestWishlistService_BookItem(t *testing.T) {
	t.Parallel()

	validEventID := uuid.New()
	validUserID := uuid.New()
	validItemID := uuid.New()

	tests := []struct {
		name       string
		role       string
		roleErr    error
		repoErr    error
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:    "Guest can book",
			role:    "guest",
			repoErr: nil,
			wantErr: false,
		},
		{
			name:       "Organizer cannot book",
			role:       "organizer",
			wantErr:    true,
			wantErrMsg: "organizer cannot book wishlist items",
		},
		{
			name:    "Role error",
			roleErr: errors.New("role error"),
			wantErr: true,
		},
		{
			name:       "Repo returns already booked",
			role:       "guest",
			repoErr:    errors.New("item already booked"),
			wantErr:    true,
			wantErrMsg: "item already booked",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repoStub := &stubWishlistRepository{saveErr: tc.repoErr}
			roleStub := &stubRoleProvider{role: tc.role, err: tc.roleErr}

			svc := NewWishlistService(repoStub, nil, roleStub)
			err := svc.BookItem(context.Background(), validEventID, validItemID, validUserID)

			if (err != nil) != tc.wantErr {
				t.Fatalf("BookItem() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr && tc.wantErrMsg != "" && err.Error() != tc.wantErrMsg {
				t.Fatalf("BookItem() expected error %v got %v", tc.wantErrMsg, err.Error())
			}
		})
	}
}

func TestWishlistService_FundItem(t *testing.T) {
	t.Parallel()

	validEventID := uuid.New()
	validUserID := uuid.New()
	validItemID := uuid.New()

	tests := []struct {
		name       string
		amount     float64
		role       string
		roleErr    error
		repoErr    error
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:    "Guest can fund",
			amount:  500.0,
			role:    "guest",
			repoErr: nil,
			wantErr: false,
		},
		{
			name:       "Zero amount",
			amount:     0.0,
			wantErr:    true,
			wantErrMsg: "amount must be greater than zero",
		},
		{
			name:       "Negative amount",
			amount:     -10.0,
			wantErr:    true,
			wantErrMsg: "amount must be greater than zero",
		},
		{
			name:       "Organizer cannot fund",
			amount:     100.0,
			role:       "organizer",
			wantErr:    true,
			wantErrMsg: "organizer cannot fund wishlist items",
		},
		{
			name:    "Role error",
			amount:  100.0,
			roleErr: errors.New("role err"),
			wantErr: true,
		},
		{
			name:       "Repo returns item booked error",
			amount:     100.0,
			role:       "guest",
			repoErr:    errors.New("item is already booked"),
			wantErr:    true,
			wantErrMsg: "item is already booked",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repoStub := &stubWishlistRepository{saveErr: tc.repoErr}
			roleStub := &stubRoleProvider{role: tc.role, err: tc.roleErr}

			svc := NewWishlistService(repoStub, nil, roleStub)
			_, err := svc.FundItem(context.Background(), validEventID, validItemID, validUserID, tc.amount)

			if (err != nil) != tc.wantErr {
				t.Fatalf("FundItem() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr && tc.wantErrMsg != "" && err.Error() != tc.wantErrMsg {
				t.Fatalf("FundItem() expected error %v got %v", tc.wantErrMsg, err.Error())
			}
		})
	}
}
