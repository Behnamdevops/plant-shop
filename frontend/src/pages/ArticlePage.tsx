import { usePublicData } from "../context/PublicDataContext";
import { useEffect, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { getArticleBySlug, type Article } from "../api/articles";
import { marked } from "marked";
import DOMPurify from "isomorphic-dompurify";
import { getProducts } from "../api/products";
import type { Product } from "../types/product";
import ProductCard from "../components/ProductCard";
import SEO from "../components/SEO";

// Configure marked to be safe - no HTML rendering
marked.setOptions({
  breaks: true,
  gfm: true,
});

function ArticleDetails() {
  const bootstrap = usePublicData();
  const { slug } = useParams<{ slug: string }>();
  const [article, setArticle] = useState<Article | null>(
    bootstrap?.article || null,
  );
  const [related, setRelated] = useState<Product[]>(
    bootstrap?.relatedProducts || [],
  );
  const [loading, setLoading] = useState(!bootstrap);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!slug) return;
    let active = true;
    getArticleBySlug(slug)
      .then((data) => {
        if (active) {
          setArticle(data);
          setError(null);
        }
      })
      .catch((err) => {
        if (active)
          setError(err instanceof Error ? err.message : "مقاله یافت نشد");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [slug]);

  useEffect(() => {
    if (!slug) return;
    let active = true;
    getProducts({ guide: slug, in_stock: true, page_size: 8 })
      .then((v) => {
        if (active) setRelated(v.items);
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, [slug]);
  const formatDate = (dateStr: string | null) => {
    if (!dateStr) return "";
    return new Date(dateStr).toLocaleDateString("fa-IR", {
      year: "numeric",
      month: "long",
      day: "numeric",
    });
  };

  const renderMarkdown = (content: string) => {
    // Parse markdown to HTML, then sanitize to prevent XSS
    const html = marked.parse(content) as string;
    // Sanitize: removes script tags, event handlers, javascript: links, etc.
    const cleanHtml = DOMPurify.sanitize(html, {
      ALLOWED_TAGS: [
        "p",
        "br",
        "strong",
        "em",
        "b",
        "i",
        "u",
        "h1",
        "h2",
        "h3",
        "h4",
        "h5",
        "h6",
        "ul",
        "ol",
        "li",
        "a",
        "blockquote",
        "code",
        "pre",
        "hr",
        "img",
        "table",
        "thead",
        "tbody",
        "tr",
        "th",
        "td",
      ],
      ALLOWED_ATTR: ["href", "src", "alt", "title", "class"],
      ALLOW_DATA_ATTR: false,
    });
    return { __html: cleanHtml };
  };

  if (loading) {
    return <div className="loading">در حال بارگذاری...</div>;
  }

  if (error || !article) {
    return (
      <div className="error-page">
        <SEO title="مقاله یافت نشد" noindex />
        <h1>مقاله یافت نشد</h1>
        <p>{error || "این مقاله ممکن است حذف شده یا منتشر نشده باشد."}</p>
        <Link to="/blog" className="back-link">
          بازگشت به مقالات
        </Link>
      </div>
    );
  }

  const structuredData = article
    ? {
        "@context": "https://schema.org",
        "@type": "Article",
        headline: article.title,
        description: article.excerpt,
        datePublished: article.published_at,
        image: article.cover_image_url,
      }
    : null;

  return (
    <>
      <SEO
        title={article?.seo_title || article?.title || "مقاله"}
        description={article?.seo_description || article?.excerpt}
        canonical={article ? `/blog/${article.slug}` : "/blog"}
      />
      {structuredData && (
        <script type="application/ld+json">
          {JSON.stringify(structuredData).replace(/</g, "\\u003c")}
        </script>
      )}
      <div className="article-page">
        <article className="article">
          {article.cover_image_url && (
            <div className="article-hero">
              <img
                src={article.cover_image_url}
                alt={article.title}
                onError={(e) => {
                  (e.target as HTMLImageElement).src = "/placeholder-image.jpg";
                }}
              />
            </div>
          )}

          <header className="article-header">
            {article.category && (
              <Link
                to={`/blog?category=${article.category.id}`}
                className="article-category"
              >
                {article.category.name}
              </Link>
            )}
            <h1>{article.title}</h1>
            <div className="article-meta">
              <span className="article-date">
                {formatDate(article.published_at)}
              </span>
            </div>
          </header>

          <div
            className="article-body markdown-content"
            dangerouslySetInnerHTML={renderMarkdown(article.content)}
          />
        </article>

        <div className="article-footer">
          <Link to="/blog" className="back-link">
            ← بازگشت به مقالات
          </Link>
        </div>
      </div>
      {related.length > 0 && (
        <section className="guide-products">
          <h2>محصولات مرتبط با این راهنما</h2>
          <p>مشخصات و مناسب‌بودن محصول را پیش از مصرف بررسی کنید.</p>
          <div className="product-grid">
            {related.map((p) => (
              <ProductCard key={p.id} product={p} />
            ))}
          </div>
        </section>
      )}
    </>
  );
}

export default function ArticlePage() {
  const { slug } = useParams();
  return <ArticleDetails key={slug} />;
}
