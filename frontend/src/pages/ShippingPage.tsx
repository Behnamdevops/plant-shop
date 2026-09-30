import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import SEO from "../components/SEO";
import { getShippingRates, type ShippingRates } from "../types/checkout";
import { usePublicData } from "../context/PublicDataContext";
import { formatToman } from "../lib/format";
export default function ShippingPage() {
  const data = usePublicData();
  const [rates, setRates] = useState<ShippingRates | null>(
      data?.shipping || null,
    ),
    [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    getShippingRates()
      .then((v) => {
        if (active) setRates(v);
      })
      .catch(() => {
        if (active)
          setError("دریافت تعرفه ارسال انجام نشد. لطفاً دوباره تلاش کنید.");
      });
    return () => {
      active = false;
    };
  }, []);
  return (
    <main className="shipping-page">
      <SEO
        title="ارسال و حمل"
        description="روش‌ها و تعرفه جاری ارسال فروشگاه"
        canonical="/shipping"
      />
      <h1>ارسال و حمل</h1>
      <section className="content-section">
        <h2>تعرفه جاری</h2>
        {error && <p role="alert">{error}</p>}
        {rates?.configured ? (
          <>
            <p>ارسال استاندارد: {formatToman(rates.standard)}</p>
            <p>ارسال اکسپرس: {formatToman(rates.express)}</p>
            {rates.free_shipping_threshold > 0 && (
              <p>
                ارسال استاندارد برای مبلغ محصولات پس از تخفیف از{" "}
                {formatToman(rates.free_shipping_threshold)} رایگان است.
              </p>
            )}
            <p>هزینه نهایی پیش از ثبت سفارش نمایش داده می‌شود.</p>
          </>
        ) : (
          <p>
            تعرفه ارسال هنوز تأیید نشده است؛ ثبت سفارش تا تکمیل تنظیمات فعال
            نمی‌شود.
          </p>
        )}
      </section>
      <section className="content-section">
        <h2>پیگیری و دریافت</h2>
        <p>
          وضعیت سفارش در حساب کاربری نمایش داده می‌شود. زمان تحویل را پیش از
          خرید با پشتیبانی هماهنگ کنید.
        </p>
        <p>
          هنگام دریافت، بسته و محصول را بررسی کنید و در صورت آسیب، عکس بسته و
          محصول را نگه دارید.
        </p>
        <Link to="/returns">درخواست بررسی آسیب و مرجوعی</Link>
      </section>
    </main>
  );
}
