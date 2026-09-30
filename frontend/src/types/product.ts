export type ProductDetails = {
  kind?: string;
  brand?: string;
  weight_volume?: string;
  formulation?: string;
  suitable_for?: string;
  composition?: string;
  usage?: string;
  benefits?: string;
  warnings?: string;
  pack_count?: number;
  country?: string;
  expiry_date?: string;
  included?: string;
  article_slug?: string;
  related_ids?: number[];
  delivery_info?: string;
};
export type Product = {
  details?: ProductDetails;
  sold_quantity?: number;
  image_urls?: string[];
  id: number;
  name: string;
  slug: string;
  description: string;
  price: number;
  stock: number;
  image_url: string | null;
  category_id: number | null;
  created_at: string;
  updated_at: string;
};

// ProductSort mirrors the backend's allow-listed sort modes
// (backend/internal/product/model.go). Never send arbitrary strings here.
export type ProductSort =
  | "newest"
  | "price_asc"
  | "price_desc"
  | "name_asc"
  | "bestselling";

// ProductListResult is the structured, paginated response shape returned by
// GET /api/v1/products.
export type ProductListResult = {
  items: Product[];
  page: number;
  page_size: number;
  total: number;
  total_pages: number;
};

export type ProductListFilters = {
  kind?: string;
  brand?: string;
  formulation?: string;
  guide?: string;
  q?: string;
  category?: number;
  in_stock?: boolean;
  min_price?: number;
  max_price?: number;
  sort?: ProductSort;
  page?: number;
  page_size?: number;
};
