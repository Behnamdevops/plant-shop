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
import AdminArticlesPage from './pages/admin/AdminArticlesPage'
import AdminArticleFormPage from './pages/admin/AdminArticleFormPage'
import AdminArticleCategoriesPage from './pages/admin/AdminArticleCategoriesPage'
import AdminDashboardPage from './pages/admin/AdminDashboardPage'
import AdminInventoryPage from './pages/admin/AdminInventoryPage'
import AccountPage from './pages/AccountPage'
import ProfilePage from './pages/ProfilePage'
import AddressesPage from './pages/AddressesPage'
import AboutPage from './pages/AboutPage'
import ArticlePage from './pages/ArticlePage'
import ArticlesPage from './pages/ArticlesPage'
import CartPage from './pages/CartPage'
import CheckoutPage from './pages/CheckoutPage'
import ContactPage from './pages/ContactPage'
import FAQPage from './pages/FAQPage'
import HomePage from './pages/HomePage'
import LoginPage from './pages/LoginPage'
import OrderDetailPage from './pages/OrderDetailPage'
import OrdersPage from './pages/OrdersPage'
import PaymentResultPage from './pages/PaymentResultPage'
import PrivacyPage from './pages/PrivacyPage'
import ProductPage from './pages/ProductPage'
import RegisterPage from './pages/RegisterPage'
import ReturnsPage from './pages/ReturnsPage'
import ShopPage from './pages/ShopPage'
import ShippingPage from './pages/ShippingPage'
import TermsPage from './pages/TermsPage'

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Header />
        <div className="page">
          <Routes>
            <Route path="/" element={<HomePage />} />
            <Route path="/shop" element={<ShopPage />} />
            <Route path="/products/:slug" element={<ProductPage />} />
            <Route path="/articles" element={<ArticlesPage />} />
            <Route path="/articles/:slug" element={<ArticlePage />} />
            <Route path="/about" element={<AboutPage />} />
            <Route path="/contact" element={<ContactPage />} />
            <Route path="/faq" element={<FAQPage />} />
            <Route path="/shipping" element={<ShippingPage />} />
            <Route path="/returns" element={<ReturnsPage />} />
            <Route path="/privacy" element={<PrivacyPage />} />
            <Route path="/terms" element={<TermsPage />} />
            <Route path="/login" element={<LoginPage />} />
            <Route path="/register" element={<RegisterPage />} />
            <Route path="/cart" element={<CartPage />} />
            <Route path="/checkout" element={<CheckoutPage />} />
            <Route path="/orders" element={<OrdersPage />} />
            <Route path="/orders/:id" element={<OrderDetailPage />} />
            <Route path="/payment/result" element={<PaymentResultPage />} />
            {/* Account V2 routes */}
            <Route path="/account" element={<AccountPage />} />
            <Route path="/account/profile" element={<ProfilePage />} />
            <Route path="/account/addresses" element={<AddressesPage />} />
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
            <Route
              path="/admin/articles"
              element={
                <AdminRoute>
                  <AdminArticlesPage />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/articles/new"
              element={
                <AdminRoute>
                  <AdminArticleFormPage />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/articles/:id/edit"
              element={
                <AdminRoute>
                  <AdminArticleFormPage />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/article-categories"
              element={
                <AdminRoute>
                  <AdminArticleCategoriesPage />
                </AdminRoute>
              }
            />
            <Route
              path="/admin"
              element={
                <AdminRoute>
                  <AdminDashboardPage />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/inventory"
              element={
                <AdminRoute>
                  <AdminInventoryPage />
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
