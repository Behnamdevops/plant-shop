import { throwApiError } from './errors'

// Dashboard metrics
export type DashboardMetrics = {
  orders_total: number
  orders_pending: number
  orders_processing: number
  orders_shipped: number
  orders_delivered: number
  users_total: number
  products_total: number
  products_low_stock: number
  products_out_of_stock: number
  paid_revenue_total: number
}

export async function getDashboardMetrics(): Promise<DashboardMetrics> {
  const response = await fetch('/api/v1/admin/dashboard', {
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

// Low stock products
export type LowStockProduct = {
  id: number
  name: string
  slug: string
  stock: number
  is_out_of_stock: boolean
}

export type LowStockResponse = {
  items: LowStockProduct[]
  page: number
  page_size: number
  total: number
  total_pages: number
}

export async function getLowStockProducts(page = 1, pageSize = 50): Promise<LowStockResponse> {
  const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) })
  const response = await fetch(`/api/v1/admin/inventory/low-stock?${params}`, {
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

// Stock adjustment
export type StockAdjustmentInput = {
  delta: number
  reason: string
}

export type StockAdjustmentResponse = {
  product_id: number
  product_name: string
  stock_before: number
  stock_after: number
  delta: number
  reason: string
  created_at: string
}

export async function adjustStock(productId: number, input: StockAdjustmentInput): Promise<StockAdjustmentResponse> {
  const response = await fetch(`/api/v1/admin/products/${productId}/stock-adjustment`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(input),
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

// Inventory adjustments history
export type InventoryAdjustment = {
  id: number
  product_id: number
  product_name: string
  product_slug: string
  admin_user_id: number | null
  admin_name: string | null
  admin_email: string | null
  delta: number
  stock_before: number
  stock_after: number
  reason: string
  created_at: string
}

export type InventoryAdjustmentList = {
  items: InventoryAdjustment[]
  page: number
  page_size: number
  total: number
  total_pages: number
}

export async function getInventoryAdjustments(
  product_id?: number,
  page = 1,
  pageSize = 20
): Promise<InventoryAdjustmentList> {
  const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) })
  if (product_id !== undefined) {
    params.set('product_id', String(product_id))
  }
  const response = await fetch(`/api/v1/admin/inventory/adjustments?${params}`, {
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}
