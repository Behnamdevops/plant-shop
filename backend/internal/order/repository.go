package order

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// cartLine is a cart item joined with its current product data, used only
// during checkout.
type cartLine struct {
	ProductID   int64
	ProductName string
	ProductSlug string
	Price       int64
	Stock       int
	Quantity    int
}

// CreateFromCart builds an order from userID's current cart plus the
// user-provided checkout input (delivery snapshot + shipping method), in a
// single transaction: it locks the relevant product rows, validates stock,
// calculates the items subtotal and server-side shipping fee, decrements
// stock, snapshots each cart line into an order_items row, and clears the
// cart. Nothing is persisted unless every step succeeds. The caller must
// have already validated input (e.g. via CheckoutInput.Validate).
func (r *Repository) CreateFromCart(ctx context.Context, userID int64, input CheckoutInput) (Order, error) {
	shippingFee, ok := ShippingFeeFor(input.ShippingMethod)
	if !ok {
		// Defense in depth: handlers validate the shipping method before
		// calling this, so reaching here means a validation gap upstream.
		return Order{}, &ErrValidation{Field: "shipping_method", Message: "is not a supported shipping method"}
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback(ctx)

	// Lock the product rows referenced by the cart so concurrent checkouts
	// or stock updates can't race with this one.
	rows, err := tx.Query(ctx, `
		SELECT ci.product_id, p.name, p.slug, p.price, p.stock, ci.quantity
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id
		WHERE ci.user_id = $1
		ORDER BY ci.product_id
		FOR UPDATE OF p
	`, userID)
	if err != nil {
		return Order{}, err
	}
	lines := make([]cartLine, 0)
	for rows.Next() {
		var l cartLine
		if err := rows.Scan(&l.ProductID, &l.ProductName, &l.ProductSlug, &l.Price, &l.Stock, &l.Quantity); err != nil {
			rows.Close()
			return Order{}, err
		}
		lines = append(lines, l)
	}
	if err := rows.Err(); err != nil {
		return Order{}, err
	}
	rows.Close()

	if len(lines) == 0 {
		return Order{}, ErrEmptyCart
	}

	for _, l := range lines {
		if l.Quantity > l.Stock {
			return Order{}, ErrInsufficientStock
		}
	}

	var itemsSubtotal int64
	for _, l := range lines {
		itemsSubtotal += l.Price * int64(l.Quantity)
	}
	total := itemsSubtotal + shippingFee

	var o Order
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (
			user_id, status, total,
			recipient_name, phone, address_line1, address_line2, city, postal_code, country,
			shipping_method, shipping_fee, items_subtotal,
			payment_status, payment_method
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, user_id, status, total, created_at, updated_at,
			recipient_name, phone, address_line1, address_line2, city, postal_code, country,
			shipping_method, shipping_fee, items_subtotal,
			payment_status, payment_method
	`,
		userID, StatusPending, total,
		input.RecipientName, input.Phone, input.AddressLine1, nullableString(input.AddressLine2), input.City, input.PostalCode, input.Country,
		input.ShippingMethod, shippingFee, itemsSubtotal,
		PaymentStatusPending, PaymentMethodManual,
	).Scan(
		&o.ID, &o.UserID, &o.Status, &o.Total, &o.CreatedAt, &o.UpdatedAt,
		&o.RecipientName, &o.Phone, &o.AddressLine1, &o.AddressLine2, &o.City, &o.PostalCode, &o.Country,
		&o.ShippingMethod, &o.ShippingFee, &o.ItemsSubtotal,
		&o.PaymentStatus, &o.PaymentMethod,
	)
	if err != nil {
		return Order{}, err
	}

	for _, l := range lines {
		subtotal := l.Price * int64(l.Quantity)
		_, err = tx.Exec(ctx, `
			INSERT INTO order_items (order_id, product_id, product_name, product_slug, unit_price, quantity, subtotal)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, o.ID, l.ProductID, l.ProductName, l.ProductSlug, l.Price, l.Quantity, subtotal)
		if err != nil {
			return Order{}, err
		}

		_, err = tx.Exec(ctx, `
			UPDATE products SET stock = stock - $1, updated_at = NOW() WHERE id = $2
		`, l.Quantity, l.ProductID)
		if err != nil {
			return Order{}, err
		}
	}

	_, err = tx.Exec(ctx, `DELETE FROM cart_items WHERE user_id = $1`, userID)
	if err != nil {
		return Order{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Order{}, err
	}
	return o, nil
}

// nullableString returns nil for an empty string and a pointer to s
// otherwise, so optional fields (currently only address_line2) are stored
// as SQL NULL rather than an empty string when not provided.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// orderColumns is the column list shared by every SELECT against orders
// that scans into an Order, kept in one place so the SELECT list and the
// Scan call below always agree.
const orderColumns = `
	id, user_id, status, total, created_at, updated_at,
	recipient_name, phone, address_line1, address_line2, city, postal_code, country,
	shipping_method, shipping_fee, items_subtotal,
	payment_status, payment_method
`

// scanOrder scans a row (in the same column order as orderColumns) into o.
func scanOrder(row pgx.Row, o *Order) error {
	return row.Scan(
		&o.ID, &o.UserID, &o.Status, &o.Total, &o.CreatedAt, &o.UpdatedAt,
		&o.RecipientName, &o.Phone, &o.AddressLine1, &o.AddressLine2, &o.City, &o.PostalCode, &o.Country,
		&o.ShippingMethod, &o.ShippingFee, &o.ItemsSubtotal,
		&o.PaymentStatus, &o.PaymentMethod,
	)
}

// ListByUser returns all orders belonging to userID, most recent first.
func (r *Repository) ListByUser(ctx context.Context, userID int64) ([]Order, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+orderColumns+`
		FROM orders
		WHERE user_id = $1
		ORDER BY id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]Order, 0)
	for rows.Next() {
		var o Order
		if err := scanOrder(rows, &o); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// GetByIDForUser returns the order (with its items) identified by orderID,
// but only if it belongs to userID. Returns ErrOrderNotFound otherwise.
func (r *Repository) GetByIDForUser(ctx context.Context, userID, orderID int64) (OrderWithItems, error) {
	var o Order
	err := scanOrder(r.db.QueryRow(ctx, `
		SELECT `+orderColumns+`
		FROM orders
		WHERE id = $1 AND user_id = $2
	`, orderID, userID), &o)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OrderWithItems{}, ErrOrderNotFound
		}
		return OrderWithItems{}, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, order_id, product_id, product_name, product_slug, unit_price, quantity, subtotal
		FROM order_items
		WHERE order_id = $1
		ORDER BY id
	`, o.ID)
	if err != nil {
		return OrderWithItems{}, err
	}
	defer rows.Close()

	items := make([]Item, 0)
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.OrderID, &it.ProductID, &it.ProductName, &it.ProductSlug, &it.UnitPrice, &it.Quantity, &it.Subtotal); err != nil {
			return OrderWithItems{}, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return OrderWithItems{}, err
	}

	return OrderWithItems{Order: o, Items: items}, nil
}

// adminOrderColumns mirrors orderColumns but prefixes order columns with
// "o." (since ListAll/GetByID join against users) and appends the customer
// identity columns.
const adminOrderColumns = `
	o.id, o.user_id, o.status, o.total, o.created_at, o.updated_at,
	o.recipient_name, o.phone, o.address_line1, o.address_line2, o.city, o.postal_code, o.country,
	o.shipping_method, o.shipping_fee, o.items_subtotal,
	o.payment_status, o.payment_method,
	u.id, u.name, u.email
`

// scanAdminOrder scans a row (in the same column order as
// adminOrderColumns) into o.
func scanAdminOrder(row pgx.Row, o *AdminOrder) error {
	return row.Scan(
		&o.ID, &o.UserID, &o.Status, &o.Total, &o.CreatedAt, &o.UpdatedAt,
		&o.RecipientName, &o.Phone, &o.AddressLine1, &o.AddressLine2, &o.City, &o.PostalCode, &o.Country,
		&o.ShippingMethod, &o.ShippingFee, &o.ItemsSubtotal,
		&o.PaymentStatus, &o.PaymentMethod,
		&o.Customer.ID, &o.Customer.Name, &o.Customer.Email,
	)
}

// ListAll returns every order in the system, newest first, along with the
// safe customer identity for each. Intended for admin use only — callers
// must enforce admin authorization before calling this.
func (r *Repository) ListAll(ctx context.Context) ([]AdminOrder, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+adminOrderColumns+`
		FROM orders o
		JOIN users u ON u.id = o.user_id
		ORDER BY o.id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]AdminOrder, 0)
	for rows.Next() {
		var o AdminOrder
		if err := scanAdminOrder(rows, &o); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// GetByID returns the order (with its items and customer identity)
// identified by orderID, regardless of which user it belongs to. Intended
// for admin use only — callers must enforce admin authorization before
// calling this. Returns ErrOrderNotFound if no such order exists.
func (r *Repository) GetByID(ctx context.Context, orderID int64) (AdminOrderWithItems, error) {
	var ao AdminOrder
	err := scanAdminOrder(r.db.QueryRow(ctx, `
		SELECT `+adminOrderColumns+`
		FROM orders o
		JOIN users u ON u.id = o.user_id
		WHERE o.id = $1
	`, orderID), &ao)
	o := AdminOrderWithItems{Order: ao.Order, Customer: ao.Customer}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AdminOrderWithItems{}, ErrOrderNotFound
		}
		return AdminOrderWithItems{}, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, order_id, product_id, product_name, product_slug, unit_price, quantity, subtotal
		FROM order_items
		WHERE order_id = $1
		ORDER BY id
	`, o.ID)
	if err != nil {
		return AdminOrderWithItems{}, err
	}
	defer rows.Close()

	items := make([]Item, 0)
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.OrderID, &it.ProductID, &it.ProductName, &it.ProductSlug, &it.UnitPrice, &it.Quantity, &it.Subtotal); err != nil {
			return AdminOrderWithItems{}, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return AdminOrderWithItems{}, err
	}

	o.Items = items
	return o, nil
}

// UpdateStatus transitions orderID to newStatus and returns the updated
// order (with items and customer identity). It re-reads the order's current
// status inside the same transaction (SELECT ... FOR UPDATE) to guard
// against a concurrent status change racing this one, and rejects the
// transition with ErrInvalidTransition if it isn't allowed from the order's
// current status. Callers must validate newStatus with IsValidStatus before
// calling this. Returns ErrOrderNotFound if no such order exists.
func (r *Repository) UpdateStatus(ctx context.Context, orderID int64, newStatus string) (AdminOrderWithItems, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return AdminOrderWithItems{}, err
	}
	defer tx.Rollback(ctx)

	var currentStatus, paymentStatus, paymentMethod string
	err = tx.QueryRow(ctx, `
		SELECT status, payment_status, payment_method FROM orders WHERE id = $1 FOR UPDATE
	`, orderID).Scan(&currentStatus, &paymentStatus, &paymentMethod)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AdminOrderWithItems{}, ErrOrderNotFound
		}
		return AdminOrderWithItems{}, err
	}

	if !CanTransition(currentStatus, newStatus) {
		return AdminOrderWithItems{}, ErrInvalidTransition
	}

	// A ZarinPal order may not advance into processing/shipped/delivered
	// until its payment has actually been verified. This is enforced here
	// (server-side, inside the same row-locked transaction as the status
	// check) rather than trusting the caller, since payment_status can
	// change concurrently via the payment callback.
	if RequiresPaidPayment(paymentMethod, newStatus) && paymentStatus != PaymentStatusPaid {
		return AdminOrderWithItems{}, ErrPaymentRequired
	}

	// Cancelling an order restores each item's quantity back to product
	// stock. This happens in the same transaction as the status update
	// itself, guarded by the `orders` row lock taken above: a second,
	// concurrent cancellation attempt on the same order blocks until this
	// transaction commits, then sees currentStatus already 'cancelled' and
	// is rejected by CanTransition (cancelled is terminal) before it ever
	// reaches this block — so stock is restored exactly once no matter how
	// many times cancellation is attempted. Stock is never restored for
	// any other transition (in particular, not for delivered, which is
	// terminal and can only be reached from 'shipped', never followed by
	// 'cancelled').
	if newStatus == StatusCancelled {
		if err := restoreOrderStock(ctx, tx, orderID); err != nil {
			return AdminOrderWithItems{}, err
		}
	}

	_, err = tx.Exec(ctx, `UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2`, newStatus, orderID)
	if err != nil {
		return AdminOrderWithItems{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return AdminOrderWithItems{}, err
	}

	// Re-read the full record (with items/customer) outside the write
	// transaction now that the update has committed.
	return r.GetByID(ctx, orderID)
}

// CancelOwnOrder lets a customer cancel their own order, restoring
// inventory via the same code path as admin cancellation. It only
// succeeds if the order belongs to userID, the state machine allows a
// transition to cancelled from the order's current status (so e.g.
// shipped/delivered/already-cancelled orders are rejected exactly like the
// admin path), and the order's payment has not already succeeded — a paid
// order requires admin-mediated cancellation/refund instead of a
// self-service customer action.
func (r *Repository) CancelOwnOrder(ctx context.Context, userID, orderID int64) (OrderWithItems, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return OrderWithItems{}, err
	}
	defer tx.Rollback(ctx)

	var currentStatus, paymentStatus string
	var ownerUserID int64
	err = tx.QueryRow(ctx, `
		SELECT status, payment_status, user_id FROM orders WHERE id = $1 FOR UPDATE
	`, orderID).Scan(&currentStatus, &paymentStatus, &ownerUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OrderWithItems{}, ErrOrderNotFound
		}
		return OrderWithItems{}, err
	}
	if ownerUserID != userID {
		// Do not distinguish "exists but belongs to someone else" from
		// "does not exist", mirroring GetByIDForUser's behavior elsewhere
		// in this package.
		return OrderWithItems{}, ErrOrderNotFound
	}

	if paymentStatus == PaymentStatusPaid {
		return OrderWithItems{}, ErrOrderNotEligibleForCancel
	}

	if !CanTransition(currentStatus, StatusCancelled) {
		return OrderWithItems{}, ErrInvalidTransition
	}

	if err := restoreOrderStock(ctx, tx, orderID); err != nil {
		return OrderWithItems{}, err
	}

	_, err = tx.Exec(ctx, `UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2`, StatusCancelled, orderID)
	if err != nil {
		return OrderWithItems{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return OrderWithItems{}, err
	}

	return r.GetByIDForUser(ctx, userID, orderID)
}

// restoreOrderStock restores the stock quantities consumed by orderID's
// items, locking the affected product rows first (in ascending product_id
// order) so it never races with a concurrent checkout's
// "FOR UPDATE OF p" lock. Shared by admin cancellation (UpdateStatus) and
// customer self-cancellation (CancelOwnOrder) so inventory is restored via
// exactly one code path.
func restoreOrderStock(ctx context.Context, tx pgx.Tx, orderID int64) error {
	itemRows, err := tx.Query(ctx, `
		SELECT product_id, quantity FROM order_items WHERE order_id = $1 ORDER BY product_id
	`, orderID)
	if err != nil {
		return err
	}
	type restoreLine struct {
		ProductID int64
		Quantity  int
	}
	var toRestore []restoreLine
	for itemRows.Next() {
		var l restoreLine
		if err := itemRows.Scan(&l.ProductID, &l.Quantity); err != nil {
			itemRows.Close()
			return err
		}
		toRestore = append(toRestore, l)
	}
	if err := itemRows.Err(); err != nil {
		return err
	}
	itemRows.Close()

	for _, l := range toRestore {
		if _, err := tx.Exec(ctx, `SELECT id FROM products WHERE id = $1 FOR UPDATE`, l.ProductID); err != nil {
			return err
		}
	}
	for _, l := range toRestore {
		if _, err := tx.Exec(ctx, `
			UPDATE products SET stock = stock + $1, updated_at = NOW() WHERE id = $2
		`, l.Quantity, l.ProductID); err != nil {
			return err
		}
	}
	return nil
}
