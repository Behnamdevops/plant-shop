import { Link } from 'react-router-dom'
import { useWishlist } from '../context/WishlistContext'
import ProductCard from '../components/ProductCard'
import SEO from '../components/SEO'
export default function WishlistPage() {
  const { products, loading, error } = useWishlist()
  return (
    <main>
      <SEO title="علاقه‌مندی‌ها" noindex />
      <h1>علاقه‌مندی‌های شما</h1>
      {loading ? (
        <p>در حال بارگذاری...</p>
      ) : error ? (
        <p role="alert">{error}</p>
      ) : !products.length ? (
        <p>
          هنوز محصولی ذخیره نکرده‌اید. <Link to="/shop">دیدن محصولات</Link>
        </p>
      ) : (
        <div className="product-grid">
          {products.map((p) => (
            <ProductCard key={p.id} product={p} />
          ))}
        </div>
      )}
    </main>
  )
}
