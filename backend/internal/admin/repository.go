package admin

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const lowStockThreshold = 5

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// DashboardMetrics returns aggregated metrics for the admin dashboard.
func (r *Repository) DashboardMetrics(ctx context.Context) (DashboardMetrics, error) {
	var m DashboardMetrics

	// Orders by status
	err := r.db.QueryRow(ctx, `
		SELECT 
			COUNT(*) FILTER (WHERE status = 'pending') as pending,
			COUNT(*) FILTER (WHERE status = 'processing') as processing,
			COUNT(*) FILTER (WHERE status = 'shipped') as shipped,
			COUNT(*) FILTER (WHERE status = 'delivered') as delivered
		FROM orders
	`).Scan(&m.OrdersPending, &m.OrdersProcessing, &m.OrdersShipped, &m.OrdersDelivered)
	if err != nil {
		return DashboardMetrics{}, err
	}

	// Total orders
	err = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM orders`).Scan(&m.OrdersTotal)
	if err != nil {
		return DashboardMetrics{}, err
	}

	// Users
	err = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&m.UsersTotal)
	if err != nil {
		return DashboardMetrics{}, err
	}

	// Products
	err = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM products`).Scan(&m.ProductsTotal)
	if err != nil {
		return DashboardMetrics{}, err
	}

	// Low stock products (stock > 0 AND stock <= threshold)
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM products 
		WHERE stock > 0 AND stock <= $1
	`, lowStockThreshold).Scan(&m.ProductsLowStock)
	if err != nil {
		return DashboardMetrics{}, err
	}

	// Out of stock products (stock = 0)
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM products WHERE stock = 0
	`).Scan(&m.ProductsOutStock)
	if err != nil {
		return DashboardMetrics{}, err
	}

	// Paid revenue (only from orders with payment_status = 'paid')
	err = r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(total), 0) FROM orders 
		WHERE payment_status = 'paid'
	`).Scan(&m.PaidRevenueTotal)
	if err != nil {
		return DashboardMetrics{}, err
	}

	return m, nil
}

// LowStockProducts returns products with low or out-of-stock status.
func (r *Repository) LowStockProducts(ctx context.Context, page, pageSize int) (LowStockResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize

	var total int64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM products 
		WHERE stock <= $1
	`, lowStockThreshold).Scan(&total)
	if err != nil {
		return LowStockResponse{}, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, name, slug, stock 
		FROM products 
		WHERE stock <= $1
		ORDER BY stock ASC, id DESC
		LIMIT $2 OFFSET $3
	`, lowStockThreshold, pageSize, offset)
	if err != nil {
		return LowStockResponse{}, err
	}
	defer rows.Close()

	items := make([]LowStockProduct, 0)
	for rows.Next() {
		var p LowStockProduct
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Stock); err != nil {
			rows.Close()
			return LowStockResponse{}, err
		}
		p.IsOutofStock = p.Stock == 0
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return LowStockResponse{}, err
	}

	totalPages := 0
	if pageSize > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return LowStockResponse{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

// AdjustStock adjusts product stock and records the adjustment.
// Returns the updated stock and the adjustment record.
func (r *Repository) AdjustStock(ctx context.Context, productID int64, delta int, reason string, adminUserID *int64) (int, error) {
	if delta == 0 {
		return 0, fmt.Errorf("delta cannot be zero")
	}

	// Bounds check reason length
	if len(reason) > 255 {
		return 0, fmt.Errorf("reason too long")
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	// Lock the product row and get current stock
	var currentStock int
	err = tx.QueryRow(ctx, `
		SELECT stock FROM products WHERE id = $1 FOR UPDATE
	`, productID).Scan(&currentStock)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, fmt.Errorf("product not found")
		}
		return 0, err
	}

	// Calculate new stock and validate it won't go negative
	newStock := currentStock + delta
	if newStock < 0 {
		return 0, fmt.Errorf("stock cannot be negative")
	}

	// Update stock
	_, err = tx.Exec(ctx, `
		UPDATE products SET stock = $1, updated_at = NOW() WHERE id = $2
	`, newStock, productID)
	if err != nil {
		return 0, err
	}

	// Record the adjustment
	var adjustmentID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO inventory_adjustments (
			product_id, admin_user_id, delta, stock_before, stock_after, reason
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, productID, adminUserID, delta, currentStock, newStock, reason).Scan(&adjustmentID)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return newStock, nil
}

// InventoryAdjustments returns a paginated list of inventory adjustments with optional product filter.
func (r *Repository) InventoryAdjustments(ctx context.Context, productID *int64, page, pageSize int) (InventoryAdjustmentList, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize

	var total int64
	var query string
	var args []any

	if productID != nil {
		query = `SELECT COUNT(*) FROM inventory_adjustments WHERE product_id = $1`
		args = append(args, *productID)
	} else {
		query = `SELECT COUNT(*) FROM inventory_adjustments`
	}

	err := r.db.QueryRow(ctx, query, args...).Scan(&total)
	if err != nil {
		return InventoryAdjustmentList{}, err
	}

	// Reset args for the main query
	args = nil
	if productID != nil {
		query = `
			SELECT ia.id, ia.product_id, p.name, p.slug, ia.admin_user_id, 
				   u.name, u.email, ia.delta, ia.stock_before, ia.stock_after, 
				   ia.reason, ia.created_at
			FROM inventory_adjustments ia
			JOIN products p ON p.id = ia.product_id
			LEFT JOIN users u ON u.id = ia.admin_user_id
			WHERE ia.product_id = $1
			ORDER BY ia.created_at DESC
			LIMIT $2 OFFSET $3
		`
		args = append(args, *productID, pageSize, offset)
	} else {
		query = `
			SELECT ia.id, ia.product_id, p.name, p.slug, ia.admin_user_id, 
				   u.name, u.email, ia.delta, ia.stock_before, ia.stock_after, 
				   ia.reason, ia.created_at
			FROM inventory_adjustments ia
			JOIN products p ON p.id = ia.product_id
			LEFT JOIN users u ON u.id = ia.admin_user_id
			ORDER BY ia.created_at DESC
			LIMIT $1 OFFSET $2
		`
		args = append(args, pageSize, offset)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return InventoryAdjustmentList{}, err
	}
	defer rows.Close()

	items := make([]StockAdjustment, 0)
	for rows.Next() {
		var a StockAdjustment
		var adminName, adminEmail any
		if err := rows.Scan(&a.ID, &a.ProductID, &a.ProductName, &a.ProductSlug,
			&a.AdminUserID, &adminName, &adminEmail, &a.Delta, &a.StockBefore,
			&a.StockAfter, &a.Reason, &a.CreatedAt); err != nil {
			rows.Close()
			return InventoryAdjustmentList{}, err
		}
		if adminName != nil {
			name := adminName.(string)
			a.AdminName = &name
		}
		if adminEmail != nil {
			email := adminEmail.(string)
			a.AdminEmail = &email
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return InventoryAdjustmentList{}, err
	}

	totalPages := 0
	if pageSize > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return InventoryAdjustmentList{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

// GetProductByID returns a product by ID.
func (r *Repository) GetProductByID(ctx context.Context, id int64) (string, string, int, error) {
	var name, slug string
	var stock int
	err := r.db.QueryRow(ctx, `
		SELECT name, slug, stock FROM products WHERE id = $1
	`, id).Scan(&name, &slug, &stock)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", "", 0, fmt.Errorf("product not found")
		}
		return "", "", 0, err
	}
	return name, slug, stock, nil
}
