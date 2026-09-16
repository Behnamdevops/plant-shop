import { useNavigate } from 'react-router-dom'
import { createProduct } from '../../api/products'
import type { ProductInput } from '../../api/products'
import ProductForm from './ProductForm'

export default function AdminProductCreatePage() {
  const navigate = useNavigate()

  const handleSubmit = async (input: ProductInput) => {
    await createProduct(input)
    navigate('/admin/products')
  }

  return (
    <main>
      <div className="form-card">
        <h1>محصول جدید</h1>
        <ProductForm submitLabel="ایجاد محصول" onSubmit={handleSubmit} />
      </div>
    </main>
  )
}
