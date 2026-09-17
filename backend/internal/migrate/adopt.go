package migrate

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
)

func Adopt(ctx context.Context, target, reference *pgx.Conn, migrations []Migration, through int) error {
	index := slices.IndexFunc(migrations, func(m Migration) bool { return m.Version == through })
	if index < 0 || through > 9 {
		return errors.New("adoption supports only an explicit historical version 001-009")
	}
	return locked(ctx, target, func() error {
		var tracked bool
		if err := target.QueryRow(ctx, "SELECT to_regclass('public.schema_migrations') IS NOT NULL").Scan(&tracked); err != nil || tracked {
			return errors.New("adoption requires a database without schema_migrations")
		}
		return locked(ctx, reference, func() error {
			if err := requireEmpty(ctx, reference); err != nil {
				return errors.New("REFERENCE_DATABASE_URL must identify a separate empty disposable database")
			}
			rtx, err := reference.Begin(ctx)
			if err != nil {
				return errors.New("cannot begin reference transaction")
			}
			defer rtx.Rollback(context.Background())
			for _, migration := range migrations[:index+1] {
				if _, err := rtx.Exec(ctx, migration.SQL); err != nil {
					return fmt.Errorf("cannot construct reference schema at migration %03d", migration.Version)
				}
			}
			expected, err := snapshot(ctx, rtx)
			if err != nil {
				return err
			}
			tx, err := target.Begin(ctx)
			if err != nil {
				return errors.New("cannot begin adoption")
			}
			defer tx.Rollback(context.Background())
			rows, err := tx.Query(ctx, "SELECT relname FROM pg_class WHERE relnamespace = 'public'::regnamespace AND relkind IN ('r','p') ORDER BY relname")
			if err != nil {
				return errors.New("cannot inspect adoption tables")
			}
			var tables []string
			for rows.Next() {
				var name string
				if err := rows.Scan(&name); err != nil {
					rows.Close()
					return errors.New("cannot inspect adoption tables")
				}
				tables = append(tables, name)
			}
			rows.Close()
			if rows.Err() != nil {
				return errors.New("cannot inspect adoption tables")
			}
			for _, table := range tables {
				if _, err := tx.Exec(ctx, "LOCK TABLE "+pgx.Identifier{"public", table}.Sanitize()+" IN ACCESS EXCLUSIVE MODE"); err != nil {
					return errors.New("cannot lock adoption tables; stop application traffic before adoption")
				}
			}
			actual, err := snapshot(ctx, tx)
			if err != nil {
				return err
			}
			if !slices.Equal(expected, actual) {
				return errors.New("schema does not exactly match requested historical version; no migrations adopted")
			}
			if through >= 7 {
				var valid bool
				if err := tx.QueryRow(ctx, "SELECT NOT EXISTS (SELECT 1 FROM orders WHERE total <> items_subtotal + shipping_fee)").Scan(&valid); err != nil || !valid {
					return errors.New("historical order subtotal backfill invariant is not satisfied")
				}
			}
			if through >= 9 {
				var valid bool
				if err := tx.QueryRow(ctx, `SELECT NOT EXISTS (SELECT 1 FROM payment_attempts a WHERE NOT EXISTS (
SELECT 1 FROM payment_attempt_events e WHERE e.attempt_id = a.id AND e.event_type IN ('backfill','created')))
AND NOT EXISTS (SELECT 1 FROM payment_attempts WHERE status = 'paid' AND (ref_id IS NULL OR ref_id <= 0))`).Scan(&valid); err != nil || !valid {
					return errors.New("historical payment backfill/preflight invariants are not satisfied")
				}
			}
			if _, err := tx.Exec(ctx, historyDDL); err != nil {
				return errors.New("cannot create adoption history")
			}
			for _, migration := range migrations[:index+1] {
				if _, err := tx.Exec(ctx, "INSERT INTO public.schema_migrations (version, name, checksum, adopted) VALUES ($1, $2, $3, true)", migration.Version, migration.Name, migration.Checksum); err != nil {
					return errors.New("cannot record adoption history")
				}
			}
			if err := tx.Commit(ctx); err != nil {
				return errors.New("adoption commit failed; inspect history before retrying")
			}
			return nil
		})
	})
}

func snapshot(ctx context.Context, tx pgx.Tx) ([]string, error) {
	rows, err := tx.Query(ctx, `
SELECT item FROM (
SELECT jsonb_build_array('relation', c.relname, c.relkind, c.relpersistence, c.relrowsecurity, c.relforcerowsecurity, c.relreplident, c.reloptions)::text item
FROM pg_class c WHERE c.relnamespace = 'public'::regnamespace
UNION ALL
SELECT jsonb_build_array('column', c.relname, a.attnum, a.attname, format_type(a.atttypid,a.atttypmod), a.attnotnull, a.attidentity, a.attgenerated, pg_get_expr(d.adbin,d.adrelid), co.collname)::text
FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid LEFT JOIN pg_attrdef d ON d.adrelid=c.oid AND d.adnum=a.attnum LEFT JOIN pg_collation co ON co.oid=a.attcollation
WHERE c.relnamespace='public'::regnamespace AND c.relkind IN ('r','p','v','m') AND a.attnum>0 AND NOT a.attisdropped
UNION ALL
SELECT jsonb_build_array('constraint', c.relname, con.conname, pg_get_constraintdef(con.oid), con.convalidated, con.condeferrable, con.condeferred)::text
FROM pg_constraint con JOIN pg_class c ON c.oid=con.conrelid WHERE c.relnamespace='public'::regnamespace
UNION ALL
SELECT jsonb_build_array('index', c.relname, pg_get_indexdef(i.indexrelid), i.indisvalid, i.indisready)::text FROM pg_index i JOIN pg_class c ON c.oid=i.indrelid WHERE c.relnamespace='public'::regnamespace
UNION ALL
SELECT jsonb_build_array('sequence', c.relname, format_type(s.seqtypid,NULL), s.seqstart, s.seqincrement, s.seqmin, s.seqmax, s.seqcache, s.seqcycle, owner.relname, a.attname, dep.deptype)::text
FROM pg_sequence s JOIN pg_class c ON c.oid=s.seqrelid LEFT JOIN pg_depend dep ON dep.classid='pg_class'::regclass AND dep.objid=c.oid AND dep.deptype IN ('a','i') LEFT JOIN pg_class owner ON owner.oid=dep.refobjid LEFT JOIN pg_attribute a ON a.attrelid=owner.oid AND a.attnum=dep.refobjsubid WHERE c.relnamespace='public'::regnamespace
UNION ALL
SELECT jsonb_build_array('function', p.proname, pg_get_functiondef(p.oid), p.proconfig, p.prosecdef)::text FROM pg_proc p WHERE p.pronamespace='public'::regnamespace
UNION ALL
SELECT jsonb_build_array('trigger', c.relname, t.tgname, pg_get_triggerdef(t.oid), t.tgenabled)::text FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid WHERE c.relnamespace='public'::regnamespace AND NOT t.tgisinternal
UNION ALL
SELECT jsonb_build_array('type', t.typname, t.typtype)::text FROM pg_type t WHERE t.typnamespace='public'::regnamespace
UNION ALL
SELECT jsonb_build_array('policy', tablename, policyname, permissive, roles, cmd, qual, with_check)::text FROM pg_policies WHERE schemaname='public'
) objects ORDER BY item`)
	if err != nil {
		return nil, errors.New("cannot inspect schema definitions")
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var item string
		if err := rows.Scan(&item); err != nil {
			return nil, errors.New("cannot inspect schema definitions")
		}
		result = append(result, item)
	}
	if rows.Err() != nil {
		return nil, errors.New("cannot inspect schema definitions")
	}
	return result, nil
}
