import type { Product } from '../types/product'
import { throwApiError } from './errors'

export type ProductInput = {
  name: string
  slug: string
  description: string
  price: number
  stock: number
  image_url: string | null
}

export async function getProducts(): Promise<Product[]> {
  const response = await fetch('/api/v1/products')

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