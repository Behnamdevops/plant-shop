// Promote an already registered account explicitly; no passwords are logged.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	email := flag.String("email", "", "email of an already registered account")
	confirm := flag.Bool("confirm", false, "confirm granting administrator privileges")
	flag.Parse()
	if *email == "" || !*confirm {
		fmt.Fprintln(os.Stderr, "Usage: go run ./cmd/admin -email you@example.com -confirm")
		os.Exit(1)
	}
	if os.Getenv("DATABASE_URL") == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot configure database")
		os.Exit(1)
	}
	defer db.Close()
	tag, err := db.Exec(ctx, "UPDATE users SET role='admin',updated_at=NOW() WHERE lower(email)=lower($1)", *email)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Database operation failed; check connection and migrations")
		os.Exit(1)
	}
	if tag.RowsAffected() != 1 {
		fmt.Fprintln(os.Stderr, "Account not found. Register on the site first.")
		os.Exit(1)
	}
	fmt.Println("Administrator role granted. Sign in again to access /admin.")
}
