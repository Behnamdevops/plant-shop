import type { Product } from '../types/product'

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