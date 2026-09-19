import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getDashboardMetrics } from '../../api/admin'
import { formatToman } from '../../lib/format'

type DashboardMetrics = {
  orders_total: number
  orders_pending: number
  orders_processing: number
  orders_shipped: number
  orders_delivered: number
  users_total: number
  products_total: number
  products_low_stock: number
  products_out_of_stock: number
  paid_revenue_total: number
}

const Card = ({ title, value, subtext, color }: { title: string; value: string | number; subtext?: string; color: string }) => {
  const colorMap: Record<string, string> = {
    blue: 'bg-blue-50 text-blue-900',
    yellow: 'bg-yellow-50 text-yellow-900',
    purple: 'bg-purple-50 text-purple-900',
    orange: 'bg-orange-50 text-orange-900',
    green: 'bg-green-50 text-green-900',
    indigo: 'bg-indigo-50 text-indigo-900',
    gray: 'bg-gray-50 text-gray-900',
    red: 'bg-red-50 text-red-900',
    stone: 'bg-stone-50 text-stone-900',
    teal: 'bg-teal-50 text-teal-900',
  }

  return (
    <div className={`p-6 rounded-lg shadow-sm border border-gray-200 ${colorMap[color] || colorMap.gray}`}>
      <p className="text-sm mb-2 opacity-80">{title}</p>
      <p className="text-2xl font-bold">{value}</p>
      {subtext && <p className="text-xs mt-2 opacity-70">{subtext}</p>}
    </div>
  )
}

export default function AdminDashboardPage() {
  const [metrics, setMetrics] = useState<DashboardMetrics | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    getDashboardMetrics()
      .then(setMetrics)
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'مشکلی در بارگذاری داشبورد پیش آمد')
      })
      .finally(() => setLoading(false))
  }, [])

  return (
    <main>
      <div className="page-header">
        <h1>داشبورد مدیریت</h1>
        <p className="page-subtitle">نمایش کلی عملکرد و وضعیت فروشگاه.</p>
      </div>

      {error && (
        <p className="alert alert-error" role="alert">
          {error}
        </p>
      )}

      {loading && <p className="state-message">در حال بارگذاری داده‌ها...</p>}

      {!loading && !error && metrics && (
        <>
          {/* Summary Cards */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: '16px', marginBottom: '32px' }}>
            <Card title="تعداد سفارش‌ها" value={metrics.orders_total} color="blue" />
            <Card title="سفارش‌های در انتظار" value={metrics.orders_pending} subtext="در حال پردازش" color="yellow" />
            <Card title="سفارش‌های در حال پردازش" value={metrics.orders_processing} subtext="در حال آماده‌سازی" color="purple" />
            <Card title="سفارش‌های ارسال‌شده" value={metrics.orders_shipped} subtext="در حال ارسال" color="orange" />
            <Card title="سفارش‌های تحویل‌شده" value={metrics.orders_delivered} subtext="موفقیت‌آمیز" color="green" />
            <Card title="تعداد کاربران" value={metrics.users_total} color="indigo" />
            <Card title="تعداد محصولات" value={metrics.products_total} color="gray" />
            <Card title="محصولات کم‌موجود" value={metrics.products_low_stock} subtext="(stock ≤ 5)" color="red" />
            <Card title="محصولات ناموجود" value={metrics.products_out_of_stock} subtext="(stock = 0)" color="stone" />
            <Card title="فروش کل (پرداخت‌شده)" value={formatToman(metrics.paid_revenue_total)} subtext=" فقط سفارش‌های پرداخت‌شده" color="teal" />
          </div>

          {/* Quick Links Section */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(250px, 1fr))', gap: '16px', marginBottom: '32px' }}>
            <Link to="/admin/products" style={{ display: 'block', padding: '20px', background: 'var(--surface)', borderRadius: 'var(--radius)', boxShadow: 'var(--shadow-sm)', border: '1px solid var(--border)', textDecoration: 'none', color: 'inherit' }}>
              <h3 style={{ marginBottom: '8px', fontSize: '1.1rem' }}>مدیریت محصولات</h3>
              <p style={{ fontSize: '0.9rem', color: 'var(--text-muted)' }}>مشاهده و ویرایش تمام محصولات فروشگاه</p>
            </Link>

            <Link to="/admin/orders" style={{ display: 'block', padding: '20px', background: 'var(--surface)', borderRadius: 'var(--radius)', boxShadow: 'var(--shadow-sm)', border: '1px solid var(--border)', textDecoration: 'none', color: 'inherit' }}>
              <h3 style={{ marginBottom: '8px', fontSize: '1.1rem' }}>مدیریت سفارش‌ها</h3>
              <p style={{ fontSize: '0.9rem', color: 'var(--text-muted)' }}>مشاهده و مدیریت وضعیت سفارش‌ها</p>
            </Link>

            <Link to="/admin/inventory" style={{ display: 'block', padding: '20px', background: 'var(--surface)', borderRadius: 'var(--radius)', boxShadow: 'var(--shadow-sm)', border: '1px solid var(--border)', textDecoration: 'none', color: 'inherit' }}>
              <h3 style={{ marginBottom: '8px', fontSize: '1.1rem' }}>مدیریت انبار</h3>
              <p style={{ fontSize: '0.9rem', color: 'var(--text-muted)' }}>کنترل موجودی و تاریخچه تنظیم</p>
            </Link>

            <Link to="/admin/coupons" style={{ display: 'block', padding: '20px', background: 'var(--surface)', borderRadius: 'var(--radius)', boxShadow: 'var(--shadow-sm)', border: '1px solid var(--border)', textDecoration: 'none', color: 'inherit' }}>
              <h3 style={{ marginBottom: '8px', fontSize: '1.1rem' }}>کوپن‌ها</h3>
              <p style={{ fontSize: '0.9rem', color: 'var(--text-muted)' }}>ایجاد و مدیریت کوپن‌های تخفیف</p>
            </Link>

            <Link to="/admin/articles" style={{ display: 'block', padding: '20px', background: 'var(--surface)', borderRadius: 'var(--radius)', boxShadow: 'var(--shadow-sm)', border: '1px solid var(--border)', textDecoration: 'none', color: 'inherit' }}>
              <h3 style={{ marginBottom: '8px', fontSize: '1.1rem' }}>مقالات</h3>
              <p style={{ fontSize: '0.9rem', color: 'var(--text-muted)' }}>مدیریت محتوای وب‌سایت</p>
            </Link>

            <Link to="/admin/categories" style={{ display: 'block', padding: '20px', background: 'var(--surface)', borderRadius: 'var(--radius)', boxShadow: 'var(--shadow-sm)', border: '1px solid var(--border)', textDecoration: 'none', color: 'inherit' }}>
              <h3 style={{ marginBottom: '8px', fontSize: '1.1rem' }}>دسته‌بندی‌ها</h3>
              <p style={{ fontSize: '0.9rem', color: 'var(--text-muted)' }}>ساختار دسته‌بندی محصولات</p>
            </Link>
          </div>

          {/* Low Stock Alert */}
          {metrics.products_low_stock > 0 && (
            <div style={{ padding: '20px', background: 'var(--danger-bg)', border: '1px solid var(--danger-border)', borderRadius: 'var(--radius)', marginBottom: '32px' }}>
              <h3 style={{ color: 'var(--danger)', fontSize: '1.1rem', marginBottom: '8px' }}>⚠️ هشدار کم‌موجودی</h3>
              <p style={{ color: 'var(--danger)', marginBottom: '16px', lineHeight: '1.6' }}>
                {metrics.products_low_stock} محصول دارای موجودی کم (≤5 واحد) و{' '}
                {metrics.products_out_of_stock} محصول ناموجود هستند. لطفاً به محصولات زیر اولویت بدهید:
              </p>
              <Link to="/admin/inventory" style={{ display: 'inline-block', padding: '10px 20px', background: 'var(--danger)', color: 'var(--primary-contrast)', borderRadius: 'var(--radius)', textDecoration: 'none', fontWeight: '600' }}>
                مشاهده محصولات کم‌موجود
              </Link>
            </div>
          )}
        </>
      )}
    </main>
  )
}
