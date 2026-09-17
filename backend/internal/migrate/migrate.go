package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

const LockID int64 = 731928410251

type Migration struct {
	Version  int
	Name     string
	Checksum string
	SQL      string
}

var filename = regexp.MustCompile(`^([0-9]+)_[a-zA-Z0-9_]+\.sql$`)

func Load(files fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, errors.New("cannot read migration directory")
	}
	var migrations []Migration
	seen := map[int]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		match := filename.FindStringSubmatch(entry.Name())
		if match == nil {
			return nil, fmt.Errorf("invalid migration filename: %s", entry.Name())
		}
		version, err := strconv.Atoi(match[1])
		if err != nil || version < 1 || seen[version] {
			return nil, errors.New("invalid or duplicate migration version")
		}
		seen[version] = true
		body, err := fs.ReadFile(files, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("cannot read migration %d", version)
		}
		text := strings.ReplaceAll(string(body), "\r\n", "\n")
		sum := sha256.Sum256([]byte(text))
		sql, err := atomicSQL(text)
		if err != nil {
			return nil, fmt.Errorf("migration %d: %w", version, err)
		}
		migrations = append(migrations, Migration{version, entry.Name(), hex.EncodeToString(sum[:]), sql})
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })
	if len(migrations) == 0 {
		return nil, errors.New("no migrations found")
	}
	return migrations, nil
}

func atomicSQL(text string) (string, error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "BEGIN;") && strings.HasSuffix(text, "COMMIT;") {
		text = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(text, "BEGIN;"), "COMMIT;"))
	}
	var token strings.Builder
	start := true
	for i := 0; i < len(text); {
		if strings.HasPrefix(text[i:], "--") {
			end := strings.IndexByte(text[i:], '\n')
			if end < 0 {
				break
			}
			i += end + 1
			continue
		}
		if strings.HasPrefix(text[i:], "/*") {
			depth := 1
			i += 2
			for i < len(text) && depth > 0 {
				if strings.HasPrefix(text[i:], "/*") {
					depth++
					i += 2
				} else if strings.HasPrefix(text[i:], "*/") {
					depth--
					i += 2
				} else {
					i++
				}
			}
			if depth != 0 {
				return "", errors.New("unterminated SQL comment")
			}
			continue
		}
		c := text[i]
		if c == '\'' || c == '"' {
			i++
			closed := false
			for i < len(text) {
				if text[i] == c {
					i++
					if i < len(text) && text[i] == c {
						i++
						continue
					}
					closed = true
					break
				}
				i++
			}
			if !closed {
				return "", errors.New("unterminated SQL string")
			}
			continue
		}
		if c == '$' {
			end := strings.IndexByte(text[i+1:], '$')
			if end >= 0 {
				tag := text[i : i+end+2]
				if regexp.MustCompile(`^\$[a-zA-Z0-9_]*\$$`).MatchString(tag) {
					stop := strings.Index(text[i+len(tag):], tag)
					if stop < 0 {
						return "", errors.New("unterminated SQL body")
					}
					i += len(tag)*2 + stop
					continue
				}
			}
		}
		if c == ';' {
			start = true
			i++
			continue
		}
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			token.Reset()
			for i < len(text) && (text[i] >= 'a' && text[i] <= 'z' || text[i] >= 'A' && text[i] <= 'Z' || text[i] == '_') {
				token.WriteByte(text[i])
				i++
			}
			if start {
				switch strings.ToUpper(token.String()) {
				case "BEGIN", "COMMIT", "END", "ROLLBACK", "START", "ABORT", "PREPARE":
					return "", errors.New("transaction control is owned by the migration runner")
				}
				start = false
			}
			continue
		}
		i++
	}
	if text == "" {
		return "", errors.New("empty migration")
	}
	return text, nil
}

func Connect(ctx context.Context, databaseURL string) (*pgx.Conn, error) {
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	cfg, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New("invalid DATABASE_URL")
	}
	cfg.RuntimeParams["search_path"] = "public"
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("cannot connect to migration database")
	}
	return conn, nil
}

func locked(ctx context.Context, conn *pgx.Conn, fn func() error) error {
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", LockID); err != nil {
		return errors.New("cannot acquire migration lock (timeout or database unavailable)")
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5e9)
		defer cancel()
		if _, err := conn.Exec(unlockCtx, "SELECT pg_advisory_unlock($1)", LockID); err != nil {
			conn.Close(unlockCtx)
		}
	}()
	return fn()
}

func Run(ctx context.Context, conn *pgx.Conn, migrations []Migration) error {
	return locked(ctx, conn, func() error {
		var tracked bool
		if err := conn.QueryRow(ctx, "SELECT to_regclass('public.schema_migrations') IS NOT NULL").Scan(&tracked); err != nil {
			return errors.New("cannot inspect migration history")
		}
		if !tracked {
			if err := requireEmpty(ctx, conn); err != nil {
				return err
			}
			if _, err := conn.Exec(ctx, historyDDL); err != nil {
				return errors.New("cannot create migration history")
			}
		}
		rows, err := conn.Query(ctx, "SELECT version, name, checksum FROM public.schema_migrations ORDER BY version")
		if err != nil {
			return errors.New("cannot read migration history")
		}
		applied := 0
		for rows.Next() {
			var version int
			var name, checksum string
			if err := rows.Scan(&version, &name, &checksum); err != nil {
				rows.Close()
				return errors.New("invalid migration history")
			}
			if applied >= len(migrations) || migrations[applied].Version != version || migrations[applied].Name != name || migrations[applied].Checksum != checksum {
				rows.Close()
				return errors.New("migration history is not an unchanged prefix of available migrations")
			}
			applied++
		}
		rows.Close()
		if rows.Err() != nil {
			return errors.New("cannot read migration history")
		}
		for _, migration := range migrations[applied:] {
			if err := apply(ctx, conn, migration); err != nil {
				return err
			}
		}
		return nil
	})
}

const historyDDL = `CREATE TABLE public.schema_migrations (
version BIGINT PRIMARY KEY, name TEXT NOT NULL UNIQUE, checksum TEXT NOT NULL,
applied_at TIMESTAMPTZ NOT NULL DEFAULT now(), adopted BOOLEAN NOT NULL DEFAULT false)`

func requireEmpty(ctx context.Context, conn *pgx.Conn) error {
	var empty bool
	err := conn.QueryRow(ctx, `SELECT NOT EXISTS (SELECT 1 FROM pg_class WHERE relnamespace = 'public'::regnamespace)
AND NOT EXISTS (SELECT 1 FROM pg_proc WHERE pronamespace = 'public'::regnamespace)
AND NOT EXISTS (SELECT 1 FROM pg_type WHERE typnamespace = 'public'::regnamespace)`).Scan(&empty)
	if err != nil || !empty {
		return errors.New("database is not empty and has no migration history; use explicit verified adoption")
	}
	return nil
}

func apply(ctx context.Context, conn *pgx.Conn, migration Migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return errors.New("cannot begin migration")
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, migration.SQL); err != nil {
		return fmt.Errorf("migration %03d failed; transaction rolled back; inspect SQL and database preconditions", migration.Version)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO public.schema_migrations (version, name, checksum) VALUES ($1, $2, $3)", migration.Version, migration.Name, migration.Checksum); err != nil {
		return fmt.Errorf("cannot record migration %03d; transaction rolled back", migration.Version)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("migration %03d commit failed; inspect history before retrying", migration.Version)
	}
	return nil
}
