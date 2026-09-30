import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { usePublicData } from "../context/PublicDataContext";
import { getProducts } from "../api/products";
import { getArticles, type Article } from "../api/articles";
import type { Product } from "../types/product";
import { storeConfig } from "../config";
import { productKinds } from "../catalog";
import ProductCard from "../components/ProductCard";
import ArticleCard from "../components/ArticleCard";
import Hero from "../components/Hero";
import SEO from "../components/SEO";
import Icon from "../components/Icon";
const problems = [
  {
    title: "برگ‌ها زرد شده‌اند؟",
    text: "پیش از خرید محصول، علت را بررسی کنید.",
    slug: "yellow-leaves",
    icon: "leaf" as const,
  },
  {
    title: "رشد گیاه کم شده؟",
    text: "نور، ریشه و برنامهٔ تغذیه را بشناسید.",
    slug: "slow-growth",
    icon: "sun" as const,
  },
  {
    title: "خاک مناسب ندارید؟",
    text: "بستر کشت را بر اساس نیاز گیاه انتخاب کنید.",
    slug: "choosing-substrate",
    icon: "grid" as const,
  },
  {
    title: "نشانه‌ای از آفت دیده‌اید؟",
    text: "شناسایی مسئله و مصرف آگاهانهٔ محصول.",
    slug: "plant-protection",
    icon: "shield" as const,
  },
];
export default function HomePage() {
  const seed = usePublicData();
  const [products, setProducts] = useState<Product[]>(
    seed?.products?.items || [],
  );
  const [articles, setArticles] = useState<Article[]>(
    seed?.articles?.items || [],
  );
  const [loading, setLoading] = useState(!seed);
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    getProducts({ sort: "bestselling", page_size: 6 })
      .then((v) => {
        if (active) setProducts(v.items);
      })
      .catch(() => {
        if (active) setError("دریافت محصولات انجام نشد. دوباره تلاش کنید.");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    getArticles({ page_size: 3 })
      .then((v) => {
        if (active) setArticles(v.items);
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, []);
  const sold = products.some((p) => (p.sold_quantity || 0) > 0);
  const structured = {
    "@context": "https://schema.org",
    "@type": "WebSite",
    name: storeConfig.name,
    url: (seed?.origin || storeConfig.publicOrigin) + "/",
    potentialAction: {
      "@type": "SearchAction",
      target:
        (seed?.origin || storeConfig.publicOrigin) +
        "/shop?q={search_term_string}",
      "query-input": "required name=search_term_string",
    },
  };
  return (
    <main className="home-page">
      <SEO
        title="فروشگاه تخصصی کود، خاک و محصولات مراقبت از گیاه"
        description={`${storeConfig.name}؛ خرید کود و تقویت‌کننده، خاک و بستر کشت، ابزار نگهداری و آموزش با مشخصات و روش مصرف.`}
        canonical="/"
      />
      <Hero />
      <section className="discovery-section">
        <div className="section-heading">
          <div>
            <span className="eyebrow">مراقبت از اینجا شروع می‌شود</span>
            <h2>محصول مناسب برای نیاز گیاهتان</h2>
          </div>
          <Link className="text-link" to="/shop">
            همهٔ محصولات <Icon name="arrow" size={18} />
          </Link>
        </div>
        <div className="discovery-grid care-category-grid">
          {productKinds.map((k) => (
            <Link
              className="discovery-card"
              key={k.value}
              to={`/category/${k.slug}`}
            >
              <span>
                <Icon name={k.icon} size={28} />
              </span>
              <h3>{k.label}</h3>
              <p>{k.description}</p>
              <Icon name="arrow" size={18} />
            </Link>
          ))}
        </div>
      </section>
      <section className="problem-section">
        <div className="section-heading">
          <div>
            <span className="eyebrow">اول شناخت، بعد انتخاب</span>
            <h2>مشکل گیاهت چیست؟</h2>
          </div>
          <Link className="text-link" to="/blog">
            شروع یادگیری <Icon name="arrow" size={18} />
          </Link>
        </div>
        <div className="discovery-grid">
          {problems.map((p) => (
            <Link className="problem-card" key={p.slug} to={`/blog/${p.slug}`}>
              <Icon name={p.icon} size={25} />
              <h3>{p.title}</h3>
              <p>{p.text}</p>
              <span>
                راهنما و محصولات مرتبط <Icon name="arrow" size={16} />
              </span>
            </Link>
          ))}
        </div>
      </section>
      <section className="collection-section">
        <div className="section-heading">
          <div>
            <span className="eyebrow">برای مراقبت روزمره</span>
            <h2>{sold ? "پرفروش‌های فروشگاه" : "محصولات فروشگاه"}</h2>
          </div>
          <Link className="text-link" to="/shop?sort=bestselling">
            مشاهدهٔ همه <Icon name="arrow" size={18} />
          </Link>
        </div>
        {loading && !products.length ? (
          <div className="product-grid">
            {[1, 2, 3].map((i) => (
              <div className="product-skeleton" key={i} />
            ))}
          </div>
        ) : (
          <div className="product-grid">
            {products.map((p) => (
              <ProductCard key={p.id} product={p} />
            ))}
          </div>
        )}
        {error && (
          <p className="alert alert-error" role="alert">
            {error}
          </p>
        )}
        {!loading && !products.length && !error && (
          <p className="empty-state">
            محصولات پس از ثبت در پنل مدیریت در این بخش نمایش داده می‌شوند.
          </p>
        )}
      </section>
      <section className="care-editorial">
        <div className="care-editorial__art" aria-hidden="true">
          <img src="/botanical-scene.svg" alt="" loading="lazy" />
        </div>
        <div>
          <span className="eyebrow">آموزش، بخشی از مراقبت است</span>
          <h2>
            برای هر نیاز،
            <br />
            یک انتخاب آگاهانه.
          </h2>
          <p>
            قبل از خرید کود یا محصول محافظتی، نیاز گیاه و اطلاعات روی محصول را
            بشناسید. راهنماها کمک می‌کنند انتخاب و مصرف دقیق‌تری داشته باشید.
          </p>
          <Link className="btn btn-primary" to="/blog">
            مطالعهٔ راهنماها <Icon name="arrow" size={18} />
          </Link>
        </div>
      </section>
      {articles.length > 0 && (
        <section className="journal-section">
          <div className="section-heading">
            <div>
              <span className="eyebrow">دفترچهٔ مراقبت</span>
              <h2>آموزش‌های جدید</h2>
            </div>
            <Link className="text-link" to="/blog">
              همهٔ آموزش‌ها <Icon name="arrow" size={18} />
            </Link>
          </div>
          <div className="articles-grid">
            {articles.map((a) => (
              <ArticleCard key={a.id} article={a} />
            ))}
          </div>
        </section>
      )}
      <section className="consultation-panel">
        <div>
          <span className="eyebrow">انتخاب با اطلاعات بیشتر</span>
          <h2>برای انتخاب محصول یا مشاوره راهنمایی می‌خواهید؟</h2>
          <p>
            شرایط گیاه و عکس آن را از مسیرهای تماس ثبت‌شده با پشتیبانی در میان
            بگذارید. جزئیات و هزینهٔ خدمات آموزشی در صفحهٔ هر خدمت آمده است.
          </p>
        </div>
        <Link className="btn btn-primary" to="/contact">
          ارتباط با پشتیبانی <Icon name="arrow" size={18} />
        </Link>
      </section>
      <section className="store-promises">
        {[
          {
            icon: "grid" as const,
            title: "مشخصات روشن",
            text: "برند، ترکیبات و وزن یا حجم",
          },
          {
            icon: "bag" as const,
            title: "هزینه‌های روشن",
            text: "نمایش مبلغ و ارسال پیش از پرداخت",
          },
          {
            icon: "shield" as const,
            title: "مصرف آگاهانه",
            text: "روش مصرف و هشدارهای محصول",
          },
          {
            icon: "user" as const,
            title: "پیگیری در حساب",
            text: "سفارش، مرجوعی و علاقه‌مندی‌ها",
          },
        ].map((i) => (
          <div key={i.title}>
            <Icon name={i.icon} size={27} />
            <strong>{i.title}</strong>
            <span>{i.text}</span>
          </div>
        ))}
      </section>
      <script type="application/ld+json">
        {JSON.stringify(structured).replace(/</g, "\\u003c")}
      </script>
    </main>
  );
}
