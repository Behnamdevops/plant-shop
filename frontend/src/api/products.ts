import type { Product, ProductListFilters, ProductListResult } from '../types/product'
import { throwApiError } from './errors'

export type ProductInput = {
  name: string
  slug: string
  description: string
  price: number
  stock: number
  image_url: string | null
  category_id: number | null
}

// buildListQuery converts filters into URLSearchParams, omitting empty/undefined
// values so the request URL stays clean and the backend's own defaults apply.
function buildListQuery(filters: ProductListFilters): string {
  const params = new URLSearchParams()

  if (filters.q) params.set('q', filters.q)
  if (filters.category) params.set('category', String(filters.category))
  if (filters.in_stock) params.set('in_stock', 'true')
  if (filters.min_price !== undefined) params.set('min_price', String(filters.min_price))
  if (filters.max_price !== undefined) params.set('max_price', String(filters.max_price))
  if (filters.sort) params.set('sort', filters.sort)
  if (filters.page) params.set('page', String(filters.page))
  if (filters.page_size) params.set('page_size', String(filters.page_size))

  const qs = params.toString()
  return qs ? `?${qs}` : ''
}

export async function getProducts(filters: ProductListFilters = {}): Promise<ProductListResult> {
  const response = await fetch(`/api/v1/products${buildListQuery(filters)}`)

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function getProduct(slug: string): Promise<Product> {
  const response = await fetch(`/api/v1/products/${slug}`)

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function getAdminProduct(id: number): Promise<Product> {
  const response = await fetch(`/api/v1/admin/products/${id}`, {
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function createProduct(input: ProductInput): Promise<Product> {
  const response = await fetch('/api/v1/admin/products', {
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

export async function updateProduct(id: number, input: ProductInput): Promise<Product> {
  const response = await fetch(`/api/v1/admin/products/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(input),
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function deleteProduct(id: number): Promise<void> {
  const response = await fetch(`/api/v1/admin/products/${id}`, {
    method: 'DELETE',
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }
}