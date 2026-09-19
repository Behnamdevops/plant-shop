import { throwApiError } from './errors'

export interface ArticleCategory {
  id: number
  name: string
  slug: string
  created_at: string
  updated_at: string
}

export interface ArticleCategoryInput {
  name: string
  slug: string
}

export interface Article {
  id: number
  title: string
  slug: string
  excerpt: string
  content: string
  cover_image_url: string | null
  category_id: number | null
  category?: ArticleCategory | null
  status: 'draft' | 'published'
  published_at: string | null
  seo_title: string | null
  seo_description: string | null
  created_at: string
  updated_at: string
}

export interface ArticleInput {
  title: string
  slug: string
  excerpt: string
  content: string
  cover_image_url?: string | null
  category_id?: number | null
  status: 'draft' | 'published'
  seo_title?: string | null
  seo_description?: string | null
}

export interface ArticleListResponse {
  items: Article[]
  page: number
  page_size: number
  total: number
  total_pages: number
}

export interface ArticleFilters {
  q?: string
  category?: number
  page?: number
  page_size?: number
}

// buildListQuery converts filters into URLSearchParams
function buildListQuery(filters: ArticleFilters): string {
  const params = new URLSearchParams()
  if (filters.q) params.set('q', filters.q)
  if (filters.category) params.set('category', String(filters.category))
  if (filters.page) params.set('page', String(filters.page))
  if (filters.page_size) params.set('page_size', String(filters.page_size))
  const qs = params.toString()
  return qs ? `?${qs}` : ''
}

// Public API
export async function getArticles(filters: ArticleFilters = {}): Promise<ArticleListResponse> {
  const response = await fetch(`/api/v1/articles${buildListQuery(filters)}`)

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function getArticleBySlug(slug: string): Promise<Article> {
  const response = await fetch(`/api/v1/articles/${slug}`)

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function getArticleCategories(): Promise<ArticleCategory[]> {
  const response = await fetch('/api/v1/article-categories')

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

// Admin API
export async function getAdminArticles(page = 1, pageSize = 20): Promise<ArticleListResponse> {
  const response = await fetch(`/api/v1/admin/articles?page=${page}&page_size=${pageSize}`, {
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function getAdminArticleById(id: number): Promise<Article> {
  const response = await fetch(`/api/v1/admin/articles/${id}`, {
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function createArticle(article: ArticleInput): Promise<Article> {
  const response = await fetch('/api/v1/admin/articles', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(article),
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function updateArticle(id: number, article: ArticleInput): Promise<Article> {
  const response = await fetch(`/api/v1/admin/articles/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(article),
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function deleteArticle(id: number): Promise<void> {
  const response = await fetch(`/api/v1/admin/articles/${id}`, {
    method: 'DELETE',
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }
}

export async function getAdminArticleCategories(): Promise<ArticleCategory[]> {
  const response = await fetch('/api/v1/admin/article-categories', {
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function createArticleCategory(category: ArticleCategoryInput): Promise<ArticleCategory> {
  const response = await fetch('/api/v1/admin/article-categories', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(category),
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function updateArticleCategory(id: number, category: ArticleCategoryInput): Promise<ArticleCategory> {
  const response = await fetch(`/api/v1/admin/article-categories/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(category),
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function deleteArticleCategory(id: number): Promise<void> {
  const response = await fetch(`/api/v1/admin/article-categories/${id}`, {
    method: 'DELETE',
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }
}

export type UploadImageResult = {
  url: string
}

export async function uploadArticleImage(file: File): Promise<UploadImageResult> {
  const formData = new FormData()
  formData.append('file', file)

  const response = await fetch('/api/v1/admin/uploads/articles', {
    method: 'POST',
    credentials: 'include',
    body: formData,
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}