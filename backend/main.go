package main

import (
 "context"
 "encoding/json"
 "log"
 "net/http"
 "os"

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
  log.Fatal("cannot connect to database:", err)
 }

 log.Println("connected to database")

 http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json")

  json.NewEncoder(w).Encode(map[string]string{
   "status":   "ok",
   "database": "connected",
  })
 })

 log.Println("server running on :8080")

 if err := http.ListenAndServe(":8080", nil); err != nil {
  log.Fatal(err)
 }
}