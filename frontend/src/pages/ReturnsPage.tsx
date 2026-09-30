import { Link } from "react-router-dom";
import SEO from "../components/SEO";
export default function ReturnsPage() {
  return (
    <main className="returns-page">
      <SEO
        title="مرجوعی و رسیدگی به آسیب"
        description="روش ثبت درخواست مرجوعی و بررسی آسیب محصول"
        canonical="/returns"
      />
      <h1>مرجوعی و رسیدگی به آسیب</h1>
      <section className="content-section">
        <h2>ثبت درخواست</h2>
        <p>
          برای سفارش پرداخت‌شده و تحویل‌گرفته، از صفحه جزئیات سفارش در حساب خود
          درخواست مرجوعی ثبت کنید. دلیل و توضیحات آسیب را بنویسید و عکس محصول و
          بسته‌بندی را برای بررسی پشتیبانی نگه دارید.
        </p>
        <p>
          درخواست توسط مدیر بررسی می‌شود. ثبت درخواست به معنی تأیید مرجوعی یا
          بازپرداخت نیست؛ نتیجه و مراحل بعدی در همان صفحه قابل پیگیری است.
        </p>
        <Link className="btn btn-primary" to="/orders">
          سفارش‌های من
        </Link>
      </section>
      <section className="content-section">
        <h2>لغو سفارش</h2>
        <p>
          لغو مستقیم فقط برای سفارش در انتظار پرداخت و بدون پرداخت فعال
          امکان‌پذیر است. برای سفارش پرداخت‌شده با پشتیبانی تماس بگیرید.
        </p>
        <Link to="/contact">تماس با پشتیبانی</Link>
      </section>
      <p>
        شرایط زمانی و هزینه‌های مرجوعی باید پیش از شروع فروش توسط فروشگاه اعلام
        شوند.
      </p>
    </main>
  );
}
