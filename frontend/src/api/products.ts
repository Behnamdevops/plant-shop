import type { Product } from '../types/product'

async function readErrorMessage(response: Response): Promise<string> {
  const text = await response.text()
  return text.trim() || `Request failed with status ${response.status}`
}

export async function getProducts(): Promise<Product[]> {
  const response = await fetch('/api/v1/products')

  if (!response.ok) {
    throw new Error('Failed to load products')
  }

  return response.json()
}

export async function getProduct(slug: string): Promise<Product> {
  const response = await fetch(`/api/v1/products/${slug}`)

  if (!response.ok) {
    throw new Error('Failed to load product')
  }

  return response.json()
}

// Admin product inputs mirror the backend's CreateProductInput / UpdateProductInput
// (backend/internal/product/model.go). The backend PUT endpoint is a full replace,
// so ProductInput carries every field for both create and update.
export type ProductInput = {
  name: string
  slug: string
  description: string
  price: number
  stock: number
  image_url: string | null
}

// NOTE: POST/PUT /api/v1/admin/products* are currently unauthenticated on the
// backend (no auth/role check exists). credentials are still sent for parity
// with the rest of the app and in case auth is added later.
export async function createProduct(input: ProductInput): Promise<Product> {
  const response = await fetch('/api/v1/admin/products', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(input),
  })

  if (!response.ok) {
    throw new Error(await readErrorMessage(response))
  }

  return response.json()
}

// There is no GET-by-id endpoint on the backend (only GetBySlug), so the admin
// edit page resolves a product by id from the full list.
export async function getProductById(id: number): Promise<Product | undefined> {
  const products = await getProducts()
  return products.find((product) => product.id === id)
}

export async function updateProduct(id: number, input: ProductInput): Promise<Product> {
  const response = await fetch(`/api/v1/admin/products/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(input),
  })

  if (!response.ok) {
    throw new Error(await readErrorMessage(response))
  }

  return response.json()
}