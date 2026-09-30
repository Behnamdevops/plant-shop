import { Link } from "react-router-dom";
import SEO from "../components/SEO";
import { storeConfig } from "../config";
export default function AboutPage() {
  return (
    <main className="about-page">
      <SEO
        title="دربارهٔ ما؛ مراقبت و تغذیهٔ گیاه"
        description="محصولات مراقبت از گیاه همراه آموزش انتخاب و مصرف؛ کود، خاک، ابزار و مشاوره."
        canonical="/about"
      />
      <div className="page-header">
        <h1>دربارهٔ {storeConfig.name}</h1>
        <p className="page-subtitle">محصول و آموزش برای بهتر مراقبت کردن.</p>
      </div>
      <div className="care-about">
        <h2>برای سلامت گیاه، انتخاب دقیق مهم است</h2>
        <p>
          فروشگاه ما بر کود و تقویت‌کننده، خاک و بستر کشت، ابزار نگهداری، محافظت
          و آموزش تمرکز دارد. اطلاعات هر محصول شامل مشخصات، مناسب‌بودن، روش مصرف
          و هشدارهای ثبت‌شده توسط فروشگاه است.
        </p>
        <p>
          راهنماهای آموزشی به شناخت نیاز گیاه کمک می‌کنند؛ انتخاب محصول پس از
          بررسی شرایط گیاه و برچسب سازنده انجام می‌شود.
        </p>
        <h2>اطلاعاتی که پیش از خرید می‌بینید</h2>
        <p>
          قیمت به تومان، موجودی، اقلام بسته، مشخصات محصول و هزینهٔ ارسال پیش از
          پرداخت نمایش داده می‌شوند. سفارش‌ها و درخواست‌های مرجوعی از حساب شما
          قابل پیگیری‌اند.
        </p>
        <Link className="btn btn-primary" to="/shop">
          مشاهدهٔ محصولات
        </Link>{" "}
        <Link className="btn btn-secondary" to="/blog">
          آموزش و راهنما
        </Link>
      </div>
    </main>
  );
}
