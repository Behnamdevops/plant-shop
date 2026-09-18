import type { Category } from '../types/category'
import { throwApiError } from './errors'

export type CategoryInput = {
  name: string
  slug: string
}

export async function getCategories(): Promise<Category[]> {
  const response = await fetch('/api/v1/categories')

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function getAdminCategories(): Promise<Category[]> {
  const response = await fetch('/api/v1/admin/categories', {
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function createCategory(input: CategoryInput): Promise<Category> {
  const response = await fetch('/api/v1/admin/categories', {
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

export async function updateCategory(id: number, input: CategoryInput): Promise<Category> {
  const response = await fetch(`/api/v1/admin/categories/${id}`, {
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

export async function deleteCategory(id: number): Promise<void> {
  const response = await fetch(`/api/v1/admin/categories/${id}`, {
    method: 'DELETE',
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }
}
