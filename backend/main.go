package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	"github.com/Behnamdevops/plant-shop/backend/internal/cart"
	"github.com/Behnamdevops/plant-shop/backend/internal/category"
	"github.com/Behnamdevops/plant-shop/backend/internal/config"
	"github.com/Behnamdevops/plant-shop/backend/internal/coupon"
	"github.com/Behnamdevops/plant-shop/backend/internal/migrate"
	"github.com/Behnamdevops/plant-shop/backend/internal/operational"
	"github.com/Behnamdevops/plant-shop/backend/internal/order"
	"github.com/Behnamdevops/plant-shop/backend/internal/payment"
	"github.com/Behnamdevops/plant-shop/backend/internal/product"
	"github.com/Behnamdevops/plant-shop/backend/internal/storage"
	"github.com/Behnamdevops/plant-shop/backend/internal/upload"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("server failed", "error", err.Error())
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	port, appEnv := cfg.Port, cfg.Environment
	isProduction := appEnv == "production"
	paymentConfig := cfg.Payment
	if isProduction {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		migrations, err := migrate.Load(os.DirFS("migrations"))
		if err != nil {
			return err
		}
		conn, err := migrate.Connect(ctx, os.Getenv("DATABASE_URL"))
		if err != nil {
			return err
		}
		err = migrate.Run(ctx, conn, migrations)
		conn.Close(context.Background())
		if err != nil {
			return err
		}
	}
	db, err := pgxpool.NewWithConfig(context.Background(), cfg.Pool)
	if err != nil {
		return errors.New("cannot initialize database pool")
	}
	defer db.Close()
	startupCtx, startupCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer startupCancel()
	if err := db.Ping(startupCtx); err != nil {
		return errors.New("cannot connect to database")
	}

	authRepository := auth.NewRepository(db)
	authHandler := auth.NewHandler(authRepository)
	// In production the app must be served over HTTPS, so session cookies
	// are marked Secure to prevent them being sent over plain HTTP. In
	// development this stays false so `go run .` over localhost HTTP keeps
	// working without extra setup.
	authHandler.SecureCookies = isProduction

	imageStore, err := storage.NewLocalStore(cfg.UploadDir, "/uploads/products")
	if err != nil {
		return err
	}

	productRepository := product.NewRepository(db)
	productHandler := product.NewHandler(productRepository, authHandler, imageStore)

	uploadHandler := upload.NewHandler(imageStore, authHandler)

	categoryRepository := category.NewRepository(db)
	categoryHandler := category.NewHandler(categoryRepository, authHandler)

	cartRepository := cart.NewRepository(db)
	cartHandler := cart.NewHandler(cartRepository, authHandler)

	orderRepository := order.NewRepository(db)
	orderHandler := order.NewHandler(orderRepository, authHandler)

	couponRepository := coupon.NewRepository(db)
	couponHandler := coupon.NewHandler(couponRepository, cartRepository, authHandler)

	var zarinpalClient payment.Client
	if paymentConfig.Enabled {
		zarinpalClient = payment.NewZarinPalClient(paymentConfig.MerchantID, paymentConfig.Sandbox)
	}
	paymentRepository := payment.NewRepository(db)
	paymentHandler := payment.NewHandler(paymentRepository, authHandler, zarinpalClient, paymentConfig.CallbackURL, paymentConfig.FrontendBaseURL)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", operational.Health)
	mux.HandleFunc("GET /healthz", operational.Health)
	mux.HandleFunc("GET /readyz", operational.Ready(db.Ping))

	trustedProxy := operational.TrustedProxy(os.Getenv("TRUSTED_PROXY_HOST"))
	limited := func(handler http.HandlerFunc) http.Handler {
		return operational.RateLimit(30, time.Minute, handler, trustedProxy)
	}

	mux.Handle("POST /api/v1/auth/register", limited(authHandler.Register))
	mux.Handle("POST /api/v1/auth/login", limited(authHandler.Login))
	mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)
	mux.HandleFunc("GET /api/v1/me", authHandler.Me)

	mux.HandleFunc("GET /api/v1/products", productHandler.List)
	mux.HandleFunc("GET /api/v1/products/{slug}", productHandler.GetBySlug)
	mux.HandleFunc("POST /api/v1/admin/products", productHandler.Create)
	mux.HandleFunc("GET /api/v1/admin/products/{id}", productHandler.GetByID)
	mux.HandleFunc("PUT /api/v1/admin/products/{id}", productHandler.Update)
	mux.HandleFunc("DELETE /api/v1/admin/products/{id}", productHandler.Delete)

	// Admin-only image upload. Rate-limited more conservatively than
	// general admin traffic since it does file I/O and image decoding.
	mux.Handle("POST /api/v1/admin/uploads/products", limited(uploadHandler.UploadProductImage))
	// Public, unauthenticated GET of previously uploaded images. Keys are
	// generated, unpredictable filenames — no directory listing, no
	// traversal, no arbitrary filesystem reads (see upload.FileServer).
	mux.Handle("GET /uploads/products/{key}", upload.FileServer(imageStore.Dir))

	mux.HandleFunc("GET /api/v1/categories", categoryHandler.List)
	mux.HandleFunc("GET /api/v1/admin/categories", categoryHandler.AdminList)
	mux.HandleFunc("POST /api/v1/admin/categories", categoryHandler.Create)
	mux.HandleFunc("PUT /api/v1/admin/categories/{id}", categoryHandler.Update)
	mux.HandleFunc("DELETE /api/v1/admin/categories/{id}", categoryHandler.Delete)

	mux.HandleFunc("GET /api/v1/cart", cartHandler.GetCart)
	mux.HandleFunc("POST /api/v1/cart/items", cartHandler.AddItem)
	mux.HandleFunc("PUT /api/v1/cart/items/{id}", cartHandler.UpdateItem)
	mux.HandleFunc("DELETE /api/v1/cart/items/{id}", cartHandler.DeleteItem)

	mux.HandleFunc("POST /api/v1/orders", orderHandler.Create)
	mux.HandleFunc("GET /api/v1/orders", orderHandler.List)
	mux.HandleFunc("GET /api/v1/orders/{id}", orderHandler.GetByID)
	mux.HandleFunc("POST /api/v1/orders/{id}/cancel", orderHandler.Cancel)

	mux.HandleFunc("GET /api/v1/admin/orders", orderHandler.AdminList)
	mux.HandleFunc("GET /api/v1/admin/orders/{id}", orderHandler.AdminGetByID)
	mux.HandleFunc("PUT /api/v1/admin/orders/{id}/status", orderHandler.AdminUpdateStatus)

	mux.Handle("POST /api/v1/coupons/preview", limited(couponHandler.Preview))
	mux.HandleFunc("GET /api/v1/admin/coupons", couponHandler.AdminList)
	mux.HandleFunc("GET /api/v1/admin/coupons/{id}", couponHandler.AdminGetByID)
	mux.HandleFunc("POST /api/v1/admin/coupons", couponHandler.AdminCreate)
	mux.HandleFunc("PUT /api/v1/admin/coupons/{id}", couponHandler.AdminUpdate)

	mux.Handle("POST /api/v1/orders/{id}/payments/zarinpal", limited(paymentHandler.RequestZarinPal))
	mux.Handle("GET /api/v1/payments/zarinpal/callback", operational.RateLimit(120, time.Minute, http.HandlerFunc(paymentHandler.Callback), trustedProxy))
	mux.HandleFunc("GET /api/v1/admin/orders/{id}/payments", paymentHandler.AdminListAttempts)
	mux.HandleFunc("GET /api/v1/admin/payments/reconciliation", paymentHandler.AdminListReconciliations)
	mux.Handle("POST /api/v1/admin/payments/{id}/reconcile", limited(paymentHandler.AdminReconcile))

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           operational.Logging(slog.Default(), mux),
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Run the server in the background so we can listen for shutdown
	// signals on the main goroutine and drain in-flight requests instead of
	// killing the process mid-request.
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("server starting", "port", port, "environment", appEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
		close(serverErrors)
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErrors:
		if err != nil {
			return errors.New("HTTP server failed")
		}
	case <-ctx.Done():
		stop()
		slog.Info("shutdown signal received, draining connections")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown timed out, forcing close")
			srv.Close()
		}
	}

	slog.Info("server stopped")
	return nil
}
