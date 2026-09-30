import { useEffect, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { getCategories } from "../api/categories";
import type { Category } from "../types/category";
import { usePublicData } from "../context/PublicDataContext";
import ShopPage from "./ShopPage";
import SEO from "../components/SEO";
export default function CategoryPage() {
  const { slug } = useParams();
  const seed = usePublicData();
  const [categories, setCategories] = useState<Category[]>(
    seed?.categories || [],
  );
  const [loading, setLoading] = useState(!seed);
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    getCategories()
      .then((v) => {
        if (active) setCategories(v);
      })
      .catch(() => {
        if (active) setError("دریافت دسته‌بندی‌ها انجام نشد.");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);
  const category = categories.find((c) => c.slug === slug);
  if (loading)
    return <p className="state-message">در حال دریافت دسته‌بندی...</p>;
  if (!category)
    return (
      <main>
        <SEO title="دسته‌بندی یافت نشد" noindex />
        <h1>دسته‌بندی یافت نشد</h1>
        <p>{error}</p>
        <Link to="/shop">همهٔ محصولات</Link>
      </main>
    );
  return <ShopPage key={slug} category={category} />;
}
