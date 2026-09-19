package migrate

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
)

func loadTest(t *testing.T, files map[string]string) []Migration {
	t.Helper()
	fs := fstest.MapFS{}
	for name, body := range files {
		fs[name] = &fstest.MapFile{Data: []byte(body)}
	}
	m, err := Load(fs)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestLoadOrderingAndValidation(t *testing.T) {
	m := loadTest(t, map[string]string{"10_ten.sql": "SELECT 10;", "002_two.sql": "SELECT 2;", "1_one.sql": "SELECT 1;"})
	if m[0].Version != 1 || m[1].Version != 2 || m[2].Version != 10 {
		t.Fatal(m)
	}
	for _, files := range []fstest.MapFS{
		{}, {"foo.sql": {Data: []byte("SELECT 1;")}},
		{"001_a.sql": {Data: []byte("SELECT 1;")}, "1_b.sql": {Data: []byte("SELECT 2;")}},
	} {
		if _, err := Load(files); err == nil {
			t.Fatal("invalid files accepted")
		}
	}
	a := loadTest(t, map[string]string{"001_a.sql": "SELECT 1;\r\n"})
	b := loadTest(t, map[string]string{"001_a.sql": "SELECT 1;\n"})
	if a[0].Checksum != b[0].Checksum {
		t.Fatal("line ending checksum mismatch")
	}
}

func TestAtomicSQL(t *testing.T) {
	for i, tc := range []struct {
		sql   string
		valid bool
	}{
		{"BEGIN; CREATE TABLE a(); COMMIT;", true},
		{"BEGIN; CREATE TABLE a();", false}, {"COMMIT;", false},
		{"CREATE TABLE a(); BEGIN;", false}, {"START TRANSACTION;", false},
		{"DO $$ BEGIN END $$;", true}, {"SELECT 'x; BEGIN';", true},
		{"/* nested /* */ */ SELECT 1;", true}, {"/* nested /* */", false},
		{"SELECT 'unterminated", false}, {"SELECT \"unterminated", false},
		{"DO $tag$ BEGIN", false}, {"-- comment\nROLLBACK;", false},
	} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			_, err := atomicSQL(tc.sql)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}

func testConn(t *testing.T) *pgx.Conn {
	t.Helper()
	raw := os.Getenv("DATABASE_URL")
	if raw == "" {
		t.Skip("DATABASE_URL not set")
	}
	cfg, err := pgx.ParseConfig(raw)
	if err != nil || !strings.HasSuffix(cfg.Database, "_test") {
		t.Fatal("migration tests require a disposable DATABASE_URL database ending in _test")
	}
	admin, err := Connect(t.Context(), raw)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("hardening_%d_test", time.Now().UnixNano())
	if _, err := admin.Exec(t.Context(), "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		admin.Close(context.Background())
		t.Fatal(err)
	}
	cfg.Database = name
	cfg.RuntimeParams["search_path"] = "public"
	conn, err := pgx.ConnectConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal("cannot connect to isolated test database")
	}
	t.Cleanup(func() {
		conn.Close(context.Background())
		_, err := admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize())
		if err != nil {
			t.Error("cannot remove isolated test database")
		}
		admin.Close(context.Background())
	})
	return conn
}

func realMigrations(t *testing.T) []Migration {
	t.Helper()
	m, err := Load(os.DirFS("../../migrations"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func historyCount(t *testing.T, conn *pgx.Conn) int {
	t.Helper()
	var count int
	if err := conn.QueryRow(t.Context(), "SELECT count(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestFreshAndIdempotent(t *testing.T) {
	conn := testConn(t)
	m := realMigrations(t)
	for range 3 {
		if err := Run(t.Context(), conn, m); err != nil {
			t.Fatal(err)
		}
	}
	if historyCount(t, conn) != 14 {
		t.Fatal("incomplete history")
	}
	if err := Run(t.Context(), conn, m[:9]); err == nil {
		t.Fatal("missing applied migration accepted")
	}
	m[0].Checksum = "changed"
	if err := Run(t.Context(), conn, m); err == nil {
		t.Fatal("checksum drift accepted")
	}
}

func TestFailedMigrationRollback(t *testing.T) {
	conn := testConn(t)
	m := loadTest(t, map[string]string{"001_ok.sql": "CREATE TABLE ok();", "002_bad.sql": "CREATE TABLE partial(); SELECT missing FROM absent;"})
	if err := Run(t.Context(), conn, m); err == nil {
		t.Fatal("bad migration accepted")
	}
	if historyCount(t, conn) != 1 {
		t.Fatal("failed migration marked applied")
	}
	var absent bool
	if err := conn.QueryRow(t.Context(), "SELECT to_regclass('partial') IS NULL").Scan(&absent); err != nil || !absent {
		t.Fatal("partial migration survived")
	}
	m[1].SQL = "CREATE TABLE partial();"
	if err := Run(t.Context(), conn, m); err != nil {
		t.Fatal(err)
	}
}

func TestExistingDatabaseRefused(t *testing.T) {
	conn := testConn(t)
	if _, err := conn.Exec(t.Context(), "CREATE TABLE legacy()"); err != nil {
		t.Fatal(err)
	}
	if err := Run(t.Context(), conn, realMigrations(t)); err == nil {
		t.Fatal("existing database not protected")
	}
}

func TestLock(t *testing.T) {
	conn := testConn(t)
	other, err := pgx.ConnectConfig(t.Context(), conn.Config())
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close(context.Background())
	if _, err := conn.Exec(t.Context(), "SELECT pg_advisory_lock($1)", LockID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel()
	if err := Run(ctx, other, realMigrations(t)); err == nil {
		t.Fatal("concurrent migration was not blocked")
	}
	if _, err := conn.Exec(t.Context(), "SELECT pg_advisory_unlock($1)", LockID); err != nil {
		t.Fatal(err)
	}
	if err := Run(t.Context(), conn, realMigrations(t)); err != nil {
		t.Fatal(err)
	}
}

func TestAdoptHistoricalPrefixes(t *testing.T) {
	for through := 1; through <= 9; through++ {
		t.Run(fmt.Sprint(through), func(t *testing.T) {
			target, reference := testConn(t), testConn(t)
			m := realMigrations(t)
			for _, migration := range m[:through] {
				if _, err := target.Exec(t.Context(), migration.SQL); err != nil {
					t.Fatal(err)
				}
			}
			if err := Adopt(t.Context(), target, reference, m, through); err != nil {
				t.Fatal(err)
			}
			if historyCount(t, target) != through {
				t.Fatal("adoption incomplete")
			}
			if err := Run(t.Context(), target, m); err != nil {
				t.Fatal(err)
			}
			if err := requireEmpty(t.Context(), reference); err != nil {
				t.Fatal("reference changes were not rolled back")
			}
		})
	}
}

func TestAdoptMismatch(t *testing.T) {
	for _, change := range []string{
		"ALTER TABLE products ALTER COLUMN price DROP NOT NULL",
		"DROP INDEX idx_payment_attempts_paid_order",
		"ALTER TABLE payment_attempts DISABLE TRIGGER payment_attempts_audit",
		"CREATE OR REPLACE FUNCTION enforce_payment_attempt_binding() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END; $$",
	} {
		t.Run(change, func(t *testing.T) {
			target, reference := testConn(t), testConn(t)
			m := realMigrations(t)
			for _, migration := range m {
				if _, err := target.Exec(t.Context(), migration.SQL); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := target.Exec(t.Context(), change); err != nil {
				t.Fatal(err)
			}
			if err := Adopt(t.Context(), target, reference, m, 9); err == nil {
				t.Fatal("schema mismatch adopted")
			}
			var absent bool
			if err := target.QueryRow(t.Context(), "SELECT to_regclass('schema_migrations') IS NULL").Scan(&absent); err != nil || !absent {
				t.Fatal("failed adoption left history")
			}
		})
	}
}
