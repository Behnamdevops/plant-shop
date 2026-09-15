package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	"github.com/Behnamdevops/plant-shop/backend/internal/product"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(context.Background()); err != nil {
		log.Fatal("cannot connect to database: ", err)
	}

	productRepository := product.NewRepository(db)
	productHandler := product.NewHandler(productRepository)

	authRepository := auth.NewRepository(db)
	authHandler := auth.NewHandler(authRepository)

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
	mux.HandleFunc("PUT /api/v1/admin/products/{id}", productHandler.Update)

	log.Println("server running on :8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
