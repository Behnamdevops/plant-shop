package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	"github.com/Behnamdevops/plant-shop/backend/internal/cart"
	"github.com/Behnamdevops/plant-shop/backend/internal/order"
	"github.com/Behnamdevops/plant-shop/backend/internal/product"
	"github.com/jackc/pgx/v5/pgxpool"
)

// appEnv values. "production" enables production-only safety behavior
// (currently: Secure session cookies). Anything else is treated as
// development, which keeps the existing local-HTTP-friendly behavior.
const appEnvProduction = "production"

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "development"
	}
	isProduction := appEnv == appEnvProduction

	db, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(context.Background()); err != nil {
		log.Fatal("cannot connect to database: ", err)
	}

	authRepository := auth.NewRepository(db)
	authHandler := auth.NewHandler(authRepository)
	// In production the app must be served over HTTPS, so session cookies
	// are marked Secure to prevent them being sent over plain HTTP. In
	// development this stays false so `go run .` over localhost HTTP keeps
	// working without extra setup.
	authHandler.SecureCookies = isProduction

	productRepository := product.NewRepository(db)
	productHandler := product.NewHandler(productRepository, authHandler)

	cartRepository := cart.NewRepository(db)
	cartHandler := cart.NewHandler(cartRepository, authHandler)

	orderRepository := order.NewRepository(db)
	orderHandler := order.NewHandler(orderRepository, authHandler)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]string{
			"status":   "ok",
			"database": "connected",
		})
	})

	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)
	mux.HandleFunc("GET /api/v1/me", authHandler.Me)

	mux.HandleFunc("GET /api/v1/products", productHandler.List)
	mux.HandleFunc("GET /api/v1/products/{slug}", productHandler.GetBySlug)
	mux.HandleFunc("POST /api/v1/admin/products", productHandler.Create)
	mux.HandleFunc("GET /api/v1/admin/products/{id}", productHandler.GetByID)
	mux.HandleFunc("PUT /api/v1/admin/products/{id}", productHandler.Update)
	mux.HandleFunc("DELETE /api/v1/admin/products/{id}", productHandler.Delete)

	mux.HandleFunc("GET /api/v1/cart", cartHandler.GetCart)
	mux.HandleFunc("POST /api/v1/cart/items", cartHandler.AddItem)
	mux.HandleFunc("PUT /api/v1/cart/items/{id}", cartHandler.UpdateItem)
	mux.HandleFunc("DELETE /api/v1/cart/items/{id}", cartHandler.DeleteItem)

	mux.HandleFunc("POST /api/v1/orders", orderHandler.Create)
	mux.HandleFunc("GET /api/v1/orders", orderHandler.List)
	mux.HandleFunc("GET /api/v1/orders/{id}", orderHandler.GetByID)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Run the server in the background so we can listen for shutdown
	// signals on the main goroutine and drain in-flight requests instead of
	// killing the process mid-request.
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("server running on :%s (env=%s)", port, appEnv)
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
			log.Fatal(err)
		}
	case <-ctx.Done():
		stop()
		log.Println("shutdown signal received, draining connections...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Println("graceful shutdown failed, forcing close: ", err)
			srv.Close()
		}
	}

	log.Println("server stopped")
}
