import { Link } from 'react-router-dom'
import SEO from '../components/SEO'
export default function NotFoundPage() {
  return (
    <main>
      <SEO title="صفحه پیدا نشد" noindex />
      <h1>این صفحه پیدا نشد</h1>
      <p>ممکن است آدرس تغییر کرده باشد.</p>
      <Link className="btn btn-primary" to="/shop">
        بازگشت به فروشگاه
      </Link>
    </main>
  )
}
