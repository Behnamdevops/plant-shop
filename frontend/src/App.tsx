import { PublicDataContext } from "./context/PublicDataContext";
import type { PublicData } from "./context/PublicDataContext";
import { lazy, Suspense } from "react";
import WishlistProvider from "./context/WishlistProvider";
import Footer from "./components/Footer";
import { BrowserRouter, Route, Routes, Navigate } from "react-router-dom";
import "./App.css";
import "./storefront.css";
import AdminRoute from "./components/AdminRoute";
import Header from "./components/Header";
import { AuthProvider } from "./context/AuthProvider";
const CategoryPage = lazy(() => import("./pages/CategoryPage"));
const AdminCategoriesPage = lazy(
  () => import("./pages/admin/AdminCategoriesPage"),
);
const AdminCouponsPage = lazy(() => import("./pages/admin/AdminCouponsPage"));
const AdminOrderDetailPage = lazy(
  () => import("./pages/admin/AdminOrderDetailPage"),
);
const AdminOrdersPage = lazy(() => import("./pages/admin/AdminOrdersPage"));
const AdminReturnsPage = lazy(() => import("./pages/admin/AdminReturnsPage"));
const AdminPaymentReconciliationPage = lazy(
  () => import("./pages/admin/AdminPaymentReconciliationPage"),
);
const AdminProductCreatePage = lazy(
  () => import("./pages/admin/AdminProductCreatePage"),
);
const AdminProductEditPage = lazy(
  () => import("./pages/admin/AdminProductEditPage"),
);
const AdminProductsPage = lazy(() => import("./pages/admin/AdminProductsPage"));
const AdminArticlesPage = lazy(() => import("./pages/admin/AdminArticlesPage"));
const AdminArticleFormPage = lazy(
  () => import("./pages/admin/AdminArticleFormPage"),
);
const AdminArticleCategoriesPage = lazy(
  () => import("./pages/admin/AdminArticleCategoriesPage"),
);
const AdminDashboardPage = lazy(
  () => import("./pages/admin/AdminDashboardPage"),
);
const AdminInventoryPage = lazy(
  () => import("./pages/admin/AdminInventoryPage"),
);
const AccountPage = lazy(() => import("./pages/AccountPage"));
const ProfilePage = lazy(() => import("./pages/ProfilePage"));
const AddressesPage = lazy(() => import("./pages/AddressesPage"));
const AboutPage = lazy(() => import("./pages/AboutPage"));
const ArticlePage = lazy(() => import("./pages/ArticlePage"));
const ArticlesPage = lazy(() => import("./pages/ArticlesPage"));
const CartPage = lazy(() => import("./pages/CartPage"));
const CheckoutPage = lazy(() => import("./pages/CheckoutPage"));
const ContactPage = lazy(() => import("./pages/ContactPage"));
const FAQPage = lazy(() => import("./pages/FAQPage"));
const HomePage = lazy(() => import("./pages/HomePage"));
const LoginPage = lazy(() => import("./pages/LoginPage"));
const OrderDetailPage = lazy(() => import("./pages/OrderDetailPage"));
const OrdersPage = lazy(() => import("./pages/OrdersPage"));
const PaymentResultPage = lazy(() => import("./pages/PaymentResultPage"));
const PrivacyPage = lazy(() => import("./pages/PrivacyPage"));
const ProductPage = lazy(() => import("./pages/ProductPage"));
const RegisterPage = lazy(() => import("./pages/RegisterPage"));
const ReturnsPage = lazy(() => import("./pages/ReturnsPage"));
const ShopPage = lazy(() => import("./pages/ShopPage"));
const ShippingPage = lazy(() => import("./pages/ShippingPage"));
const TermsPage = lazy(() => import("./pages/TermsPage"));
import PrivatePageProtection from "./components/PrivatePageProtection";

export function AppContent() {
  return (
    <>
      <AuthProvider>
        <WishlistProvider>
          <a className="skip-link" href="#page-content">
            رفتن به محتوای صفحه
          </a>
          <Header isPublic />
          <PrivatePageProtection />
          <div className="page" id="page-content" tabIndex={-1}>
            <Suspense
              fallback={<p className="state-message">در حال بارگذاری...</p>}
            >
              <Routes>
                <Route path="/" element={<HomePage />} />
                <Route path="/shop" element={<ShopPage />} />
                <Route path="/products/:slug" element={<ProductPage />} />
                <Route path="/blog" element={<ArticlesPage />} />
                <Route
                  path="/articles"
                  element={<Navigate to="/blog" replace />}
                />
                <Route path="/category/:slug" element={<CategoryPage />} />
                <Route path="/blog/:slug" element={<ArticlePage />} />
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
                  path="/admin/returns"
                  element={
                    <AdminRoute>
                      <AdminReturnsPage />
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
                <Route path="/wishlist" element={<WishlistPage />} />
                <Route
                  path="/forgot-password"
                  element={<ForgotPasswordPage />}
                />
                <Route path="/reset-password" element={<ResetPasswordPage />} />
                <Route
                  path="/admin/reviews"
                  element={
                    <AdminRoute>
                      <AdminReviewsPage />
                    </AdminRoute>
                  }
                />
                <Route path="*" element={<NotFoundPage />} />
              </Routes>
            </Suspense>
          </div>
          <Footer />
        </WishlistProvider>
      </AuthProvider>
    </>
  );
}

const WishlistPage = lazy(() => import("./pages/WishlistPage"));
const ForgotPasswordPage = lazy(() => import("./pages/ForgotPasswordPage"));
const ResetPasswordPage = lazy(() => import("./pages/ResetPasswordPage"));
const AdminReviewsPage = lazy(() => import("./pages/admin/AdminReviewsPage"));
const NotFoundPage = lazy(() => import("./pages/NotFoundPage"));
export default function App({
  initialData = null,
}: {
  initialData?: PublicData | null;
}) {
  return (
    <BrowserRouter>
      <PublicDataContext.Provider value={initialData}>
        <AppContent />
      </PublicDataContext.Provider>
    </BrowserRouter>
  );
}
