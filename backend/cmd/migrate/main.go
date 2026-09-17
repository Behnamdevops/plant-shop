package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/Behnamdevops/plant-shop/backend/internal/migrate"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	dir := flag.String("dir", "migrations", "migration directory")
	adopt := flag.Int("adopt-through", 0, "verify and adopt historical migrations through this version; requires REFERENCE_DATABASE_URL")
	flag.Parse()
	if err := run(*dir, *adopt); err != nil {
		slog.Error("migration command failed", "error", err.Error())
		os.Exit(1)
	}
	slog.Info("migrations complete")
}

func run(dir string, through int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	migrations, err := migrate.Load(os.DirFS(dir))
	if err != nil {
		return err
	}
	conn, err := migrate.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	if through != 0 {
		reference, err := migrate.Connect(ctx, os.Getenv("REFERENCE_DATABASE_URL"))
		if err != nil {
			return err
		}
		defer reference.Close(context.Background())
		return migrate.Adopt(ctx, conn, reference, migrations, through)
	}
	return migrate.Run(ctx, conn, migrations)
}
