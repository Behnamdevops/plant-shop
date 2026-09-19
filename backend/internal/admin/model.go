package admin

import (
	"time"
)

// DashboardMetrics represents aggregated metrics for the admin dashboard.
type DashboardMetrics struct {
	OrdersTotal      int64 `json:"orders_total"`
	OrdersPending    int64 `json:"orders_pending"`
	OrdersProcessing int64 `json:"orders_processing"`
	OrdersShipped    int64 `json:"orders_shipped"`
	OrdersDelivered  int64 `json:"orders_delivered"`
	UsersTotal       int64 `json:"users_total"`
	ProductsTotal    int64 `json:"products_total"`
	ProductsLowStock int64 `json:"products_low_stock"`
	ProductsOutStock int64 `json:"products_out_of_stock"`
	PaidRevenueTotal int64 `json:"paid_revenue_total"`
}

// LowStockProduct represents a product with low or out-of-stock status.
type LowStockProduct struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Stock        int    `json:"stock"`
	IsOutofStock bool   `json:"is_out_of_stock"`
}

// StockAdjustmentInput represents the request body for stock adjustment.
type StockAdjustmentInput struct {
	Delta  int    `json:"delta"`
	Reason string `json:"reason"`
}

// StockAdjustment represents a recorded stock adjustment.
type StockAdjustment struct {
	ID          int64     `json:"id"`
	ProductID   int64     `json:"product_id"`
	ProductName string    `json:"product_name"`
	ProductSlug string    `json:"product_slug"`
	AdminUserID *int64    `json:"admin_user_id"`
	AdminName   *string   `json:"admin_name"`
	AdminEmail  *string   `json:"admin_email"`
	Delta       int       `json:"delta"`
	StockBefore int       `json:"stock_before"`
	StockAfter  int       `json:"stock_after"`
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}

// InventoryAdjustmentList is the response for listing inventory adjustments.
type InventoryAdjustmentList struct {
	Items      []StockAdjustment `json:"items"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	Total      int64             `json:"total"`
	TotalPages int               `json:"total_pages"`
}

// LowStockResponse is the response for listing low-stock products.
type LowStockResponse struct {
	Items      []LowStockProduct `json:"items"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	Total      int64             `json:"total"`
	TotalPages int               `json:"total_pages"`
}

// StockAdjustmentResponse is the response for a stock adjustment operation.
type StockAdjustmentResponse struct {
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
	StockBefore int    `json:"stock_before"`
	StockAfter  int    `json:"stock_after"`
	Delta       int    `json:"delta"`
	Reason      string `json:"reason"`
	CreatedAt   string `json:"created_at"`
}
