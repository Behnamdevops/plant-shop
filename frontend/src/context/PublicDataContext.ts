import { createContext, useContext } from "react";
import { useLocation } from "react-router-dom";
import type { Product, ProductListResult } from "../types/product";
import type { Article, ArticleCategory } from "../api/articles";
import type { Category } from "../types/category";
import type { ShippingRates } from "../types/checkout";
export type PublicData = {
  path: string;
  origin: string;
  categories?: Category[];
  products?: ProductListResult;
  relatedProducts?: Product[];
  product?: Product;
  article?: Article;
  articles?: { items: Article[]; total_pages: number };
  articleCategories?: ArticleCategory[];
  shipping?: ShippingRates;
};
export const PublicDataContext = createContext<PublicData | null>(null);
export function usePublicData() {
  const data = useContext(PublicDataContext),
    location = useLocation();
  return data?.path === location.pathname + location.search ? data : null;
}
