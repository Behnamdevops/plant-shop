export type Product = {
  id: number
  name: string
  slug: string
  description: string
  price: number
  stock: number
  image_url: string | null
  category_id: number | null
  created_at: string
  updated_at: string
}

// ProductSort mirrors the backend's allow-listed sort modes
// (backend/internal/product/model.go). Never send arbitrary strings here.
export type ProductSort = 'newest' | 'price_asc' | 'price_desc' | 'name_asc'

// ProductListResult is the structured, paginated response shape returned by
// GET /api/v1/products.
export type ProductListResult = {
  items: Product[]
  page: number
  page_size: number
  total: number
  total_pages: number
}

export type ProductListFilters = {
  q?: string
  category?: number
  in_stock?: boolean
  min_price?: number
  max_price?: number
  sort?: ProductSort
  page?: number
  page_size?: number
}