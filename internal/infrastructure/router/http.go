package router

import (
	"log/slog"
	"net/http"

	"eventAI/internal/adapters/api/action"
	apimiddleware "eventAI/internal/adapters/api/middleware"
	"eventAI/internal/config"
	"eventAI/internal/infrastructure/ai"
	"eventAI/internal/infrastructure/authz"
	"eventAI/internal/infrastructure/database"
	"eventAI/internal/infrastructure/file"
	"eventAI/internal/infrastructure/jwt"
	"eventAI/internal/infrastructure/max"
	"eventAI/internal/repo"
	"eventAI/internal/service"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func New(cfg config.Config, logger *slog.Logger, db *pgxpool.Pool) (http.Handler, error) {
	healthHandler := action.NewHealthHandler(db)
	docsHandler := action.NewDocsHandler()

	eventRepo := database.NewEventRepository(db)
	eventService := service.NewEventService(eventRepo)
	eventHandler := action.NewEventHandler(eventService)

	authRepo := database.NewAuthRepository(db)

	wishlistRepo := repo.NewWishlistRepository(db)
	mockAIParser := ai.NewMockWishlistParser()
	wishlistService := service.NewWishlistService(wishlistRepo, mockAIParser, eventRepo)
	wishlistHandler := action.NewWishlistHandler(wishlistService)

	photoRepo := repo.NewPhotoRepository(db)
	storageProvider := file.NewLocalStorageProvider("./uploads")
	photoService := service.NewPhotoService(photoRepo, storageProvider, eventRepo)
	photoHandler := action.NewPhotoHandler(photoService)

	maxClient, err := max.NewClient(cfg.MAX.BotToken, cfg.MAX.APIBaseURL)
	if err != nil {
		return nil, err
	}

	jwtManager, err := jwt.NewManager(cfg.JWT.Secret, cfg.JWT.TTL)
	if err != nil {
		return nil, err
	}

	authService := service.NewAuthService(authRepo, maxClient, jwtManager, cfg.MAX.BotUsername)
	authHandler := action.NewAuthHandler(authService)

	authorizer, err := authz.New(cfg.Casbin.ModelPath, cfg.Casbin.PolicyPath, eventRepo)
	if err != nil {
		return nil, err
	}

	authMiddleware := apimiddleware.Auth(func(token string) (string, error) {
		claims, err := jwtManager.Verify(token)
		if err != nil {
			return "", err
		}

		return claims.Subject, nil
	})

	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(apimiddleware.Logger(logger))
	r.Use(apimiddleware.Recoverer(logger))

	docsFS := http.FileServer(http.Dir("./swag"))
	r.Get("/api/docs", docsHandler.Redirect)
	r.Get("/api/docs/", docsHandler.Index)
	r.Handle("/api/docs/*", http.StripPrefix("/api/docs/", docsFS))

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/healthz", healthHandler.Get)

		// Serve static uploaded files
		fs := http.FileServer(http.Dir("./uploads"))
		r.Handle("/uploads/*", http.StripPrefix("/uploads/", fs))

		r.Route("/auth", func(r chi.Router) {
			r.Post("/max/start", authHandler.StartMAXAuth)
			r.Post("/max/login", authHandler.LoginWithMAX)
			r.Post("/max/complete", authHandler.CompleteMAXAuth)
			r.Get("/max/session/{sessionID}", authHandler.GetMAXAuthSession)
			r.Post("/max/exchange", authHandler.ExchangeMAXAuth)
		})

		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)

			r.Patch("/me", authHandler.UpdateMe)
			r.Get("/events/my", eventHandler.ListMine)
			r.Post("/events", eventHandler.Create)
			r.Post("/events/join-by-token", eventHandler.JoinByToken)

			r.Route("/events/{eventID}", func(r chi.Router) {
				r.Use(apimiddleware.RequireEventAccess(authorizer, "read"))
				r.Get("/", eventHandler.GetByID)
				r.Get("/invite", eventHandler.GetInvite)
				r.Get("/guests", eventHandler.ListGuests)
				r.Get("/stats", eventHandler.GetGuestStats)
				r.Patch("/guests/{guestID}/status", eventHandler.UpdateGuestStatus)

				// Wishlist
				r.Get("/wishlist", wishlistHandler.GetWishlist)
				r.Post("/wishlist/parse", wishlistHandler.ParseWishlist)
				r.Post("/wishlist/ideas", wishlistHandler.SubmitGuestIdea)
				r.Post("/wishlist/{itemID}/book", wishlistHandler.BookItem)
				r.Post("/wishlist/{itemID}/fund", wishlistHandler.FundItem)

				// Photos
				r.Post("/photos/upload", photoHandler.UploadPhotos)
				r.Get("/photos", photoHandler.GetPhotos)
			})
		})
	})

	return r, nil
}
