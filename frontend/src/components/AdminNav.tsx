import { NavLink } from 'react-router-dom'
import Icon from './Icon'
export default function AdminNav() {
  return (
    <aside className="admin-nav">
      <strong>
        <Icon name="grid" /> مدیریت فروشگاه
      </strong>
      <nav aria-label="بخش‌های مدیریت">
        {[
          ['/admin', 'داشبورد'],
          ['/admin/products', 'محصولات'],
          ['/admin/categories', 'دسته‌بندی‌ها'],
          ['/admin/orders', 'سفارش‌ها'],
          ['/admin/inventory', 'موجودی و انبار'],
          ['/admin/coupons', 'کدهای تخفیف'],
          ['/admin/reviews', 'نظرات خریداران'],
          ['/admin/articles', 'مقالات'],
          ['/admin/article-categories', 'دسته‌های مقاله'],
          ['/admin/returns', 'مرجوعی‌ها'],
          ['/admin/payments/reconciliation', 'تطبیق پرداخت'],
        ].map(([to, label]) => (
          <NavLink key={to} to={to} end={to === '/admin'}>
            {label}
          </NavLink>
        ))}
      </nav>
    </aside>
  )
}
