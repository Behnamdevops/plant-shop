import { useEffect, useState } from 'react'
import { getLowStockProducts, getInventoryAdjustments, adjustStock } from '../../api/admin'
import type { StockAdjustmentInput } from '../../api/admin'

type LowStockProduct = {
  id: number
  name: string
  slug: string
  stock: number
  is_out_of_stock: boolean
}

type InventoryAdjustment = {
  id: number
  product_id: number
  product_name: string
  product_slug: string
  admin_user_id: number | null
  admin_name: string | null
  admin_email: string | null
  delta: number
  stock_before: number
  stock_after: number
  reason: string
  created_at: string
}

export default function AdminInventoryPage() {
  const [lowStock, setLowStock] = useState<LowStockProduct[]>([])
  const [adjustments, setAdjustments] = useState<InventoryAdjustment[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [stockAdjustment, setStockAdjustment] = useState<{ [key: number]: Partial<StockAdjustmentInput> }>({})

  useEffect(() => {
    Promise.all([
      getLowStockProducts(1, 50),
      getInventoryAdjustments(undefined, 1, 10),
    ])
      .then(([lowStockRes, adjRes]) => {
        setLowStock(lowStockRes.items)
        setAdjustments(adjRes.items)
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'مشکلی در بارگذاری انبار پیش آمد')
      })
      .finally(() => setLoading(false))
  }, [])

  const handleStockAdjustment = async (productId: number) => {
    const input = stockAdjustment[productId]
    if (!input || input.delta === 0 || !input.reason?.trim()) {
      setError('لطفاً مقدار و دلیل صحیح را وارد کنید')
      return
    }

    try {
      await adjustStock(productId, input as StockAdjustmentInput)
      setStockAdjustment((prev) => {
        const next = { ...prev }
        delete next[productId]
        return next
      })

      // Reload data
      await Promise.all([
        getLowStockProducts(1, 50).then((res) => setLowStock(res.items)),
        getInventoryAdjustments(undefined, 1, 10).then((res) => setAdjustments(res.items)),
      ])
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'مشکلی در تنظیم موجودی پیش آمد')
    }
  }

  const handleStockDeltaChange = (productId: number, delta: number) => {
    setStockAdjustment((prev) => ({
      ...prev,
      [productId]: {
        ...(prev[productId] || {}),
        delta,
      },
    }))
  }

  const handleStockReasonChange = (productId: number, reason: string) => {
    setStockAdjustment((prev) => ({
      ...prev,
      [productId]: {
        ...(prev[productId] || {}),
        reason,
      },
    }))
  }

  const renderStockStatus = (stock: number) => {
    if (stock === 0) {
      return <span className="badge badge-out-of-stock">ناموجود</span>
    } else if (stock <= 5) {
      return <span className="badge badge-limited-stock">کم‌موجود ({stock})</span>
    } else {
      return <span className="badge badge-in-stock">به اندازه کافی</span>
    }
  }

  return (
    <main>
      <div className="page-header">
        <h1>مدیریت انبار</h1>
        <p className="page-subtitle">کنترل موجودی و تاریخچه تنظیم محصولات.</p>
      </div>

      {error && (
        <p className="alert alert-error" role="alert">
          {error}
        </p>
      )}

      {loading && <p className="state-message">در حال بارگذاری...</p>}

      {!loading && !error && (
        <>
          {/* Low Stock Section */}
          <section className="mb-8">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-semibold text-gray-900">محصولات کم‌موجود و ناموجود</h2>
              <span className="text-sm text-gray-600">
                {lowStock.filter((p) => p.stock === 0).length} ناموجود، {lowStock.filter((p) => p.stock > 0 && p.stock <= 5).length} کم‌موجود
              </span>
            </div>

            {lowStock.length === 0 ? (
              <div className="table-wrap">
                <p className="state-message">همه محصولات موجودی کافی دارند.</p>
              </div>
            ) : (
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>نام محصول</th>
                      <th>موجودی</th>
                      <th>وضعیت</th>
                      <th>عملیات</th>
                    </tr>
                  </thead>
                  <tbody>
                    {lowStock.map((product) => (
                      <tr key={product.id}>
                        <td>
                          <div>{product.name}</div>
                          <div className="text-sm text-gray-500">slug: {product.slug}</div>
                        </td>
                        <td className={product.stock === 0 ? 'text-red-600 font-bold' : 'text-yellow-600 font-bold'}>
                          {product.stock}
                        </td>
                        <td>{renderStockStatus(product.stock)}</td>
                        <td>
                          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                            <input
                              type="number"
                              placeholder="مقدار"
                              style={{ width: '80px', padding: '6px 8px', border: '1px solid var(--border)', borderRadius: 'var(--radius)' }}
                              value={stockAdjustment[product.id]?.delta === 0 ? '' : stockAdjustment[product.id]?.delta}
                              onChange={(e) => handleStockDeltaChange(product.id, Number(e.target.value))}
                            />
                            <select
                              style={{ padding: '6px 8px', border: '1px solid var(--border)', borderRadius: 'var(--radius)' }}
                              onChange={(e) => handleStockReasonChange(product.id, e.target.value)}
                              value={stockAdjustment[product.id]?.reason || ''}
                            >
                              <option value="">دلیل</option>
                              <option value="restock">تامین انبار</option>
                              <option value="manual_correction">اصلاح دستی</option>
                              <option value="loss">از دست رفته</option>
                              <option value="damaged">معیوب</option>
                              <option value="correction">تصحیح خطا</option>
                            </select>
                            <button
                              type="button"
                              className="btn btn-primary btn-sm"
                              onClick={() => handleStockAdjustment(product.id)}
                            >
                              تنظیم
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>

          {/* Recent Adjustments Section */}
          <section>
            <h2 className="text-lg font-semibold text-gray-900 mb-4">تاریخچه تنظیمات اخی��</h2>

            {adjustments.length === 0 ? (
              <div className="table-wrap">
                <p className="state-message">هنوز تنظیم موجودی ثبت نشده است.</p>
              </div>
            ) : (
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>تاریخ</th>
                      <th>محصول</th>
                      <th>تغییر</th>
                      <th>قبل/بعد</th>
                      <th>دلیل</th>
                      <th>مدیر</th>
                    </tr>
                  </thead>
                  <tbody>
                    {adjustments.map((adj) => (
                      <tr key={adj.id}>
                        <td>
                          <div>{new Date(adj.created_at).toLocaleString('fa-IR')}</div>
                        </td>
                        <td>
                          <div>{adj.product_name}</div>
                          <div className="text-sm text-gray-500">{adj.product_slug}</div>
                        </td>
                        <td>
                          <span className={`badge ${adj.delta > 0 ? 'badge-in-stock' : 'badge-out-of-stock'}`}>
                            {adj.delta > 0 ? '+' : ''}{adj.delta}
                          </span>
                        </td>
                        <td>{adj.stock_before} → {adj.stock_after}</td>
                        <td>{adj.reason}</td>
                        <td>
                          <div>{adj.admin_name || 'سیستم'}</div>
                          {adj.admin_email && <div className="text-sm text-gray-500">{adj.admin_email}</div>}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>
        </>
      )}
    </main>
  )
}
