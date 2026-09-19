import { BrowserRouter, Route, Routes } from 'react-router-dom'
import './App.css'
import AdminRoute from './components/AdminRoute'
import Header from './components/Header'
import { AuthProvider } from './context/AuthProvider'
import AdminCategoriesPage from './pages/admin/AdminCategoriesPage'
import AdminCouponsPage from './pages/admin/AdminCouponsPage'
import AdminOrderDetailPage from './pages/admin/AdminOrderDetailPage'
import AdminOrdersPage from './pages/admin/AdminOrdersPage'
import AdminPaymentReconciliationPage from './pages/admin/AdminPaymentReconciliationPage'
import AdminProductCreatePage from './pages/admin/AdminProductCreatePage'
import AdminProductEditPage from './pages/admin/AdminProductEditPage'
import AdminProductsPage from './pages/admin/AdminProductsPage'
import CartPage from './pages/CartPage'
import CheckoutPage from './pages/CheckoutPage'
import HomePage from './pages/HomePage'
import LoginPage from './pages/LoginPage'
import OrderDetailPage from './pages/OrderDetailPage'
import OrdersPage from './pages/OrdersPage'
import PaymentResultPage from './pages/PaymentResultPage'
import ProductPage from './pages/ProductPage'
import RegisterPage from './pages/RegisterPage'

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Header />
        <div className="page">
          <Routes>
            <Route path="/" element={<HomePage />} />
            <Route path="/products/:slug" element={<ProductPage />} />
            <Route path="/login" element={<LoginPage />} />
            <Route path="/register" element={<RegisterPage />} />
            <Route path="/cart" element={<CartPage />} />
            <Route path="/checkout" element={<CheckoutPage />} />
            <Route path="/orders" element={<OrdersPage />} />
            <Route path="/orders/:id" element={<OrderDetailPage />} />
            <Route path="/payment/result" element={<PaymentResultPage />} />
            <Route
              path="/admin/products"
              element={
                <AdminRoute>
                  <AdminProductsPage />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/products/new"
              element={
                <AdminRoute>
                  <AdminProductCreatePage />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/products/:id/edit"
              element={
                <AdminRoute>
                  <AdminProductEditPage />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/categories"
              element={
                <AdminRoute>
                  <AdminCategoriesPage />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/coupons"
              element={
                <AdminRoute>
                  <AdminCouponsPage />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/payments/reconciliation"
              element={
                <AdminRoute>
                  <AdminPaymentReconciliationPage />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/orders"
              element={
                <AdminRoute>
                  <AdminOrdersPage />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/orders/:id"
              element={
                <AdminRoute>
                  <AdminOrderDetailPage />
                </AdminRoute>
              }
            />
          </Routes>
        </div>
      </AuthProvider>
    </BrowserRouter>
  )
}

export default App