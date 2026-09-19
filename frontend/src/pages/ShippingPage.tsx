import { useEffect } from 'react'
import { Link } from 'react-router-dom'
import { storeConfig } from '../config'
import SEO from '../components/SEO'

export default function ShippingPage() {
  const pageTitle = 'ارسال و حمل'
  const pageDescription = 'اطلاعات درباره ارسال محصولات گیاه و هزینه‌های مربوطه'

  useEffect(() => {
    document.title = `${pageTitle} - ${storeConfig.name}`
    const descMeta = document.querySelector('meta[name="description"]')
    if (descMeta) {
      descMeta.setAttribute('content', pageDescription)
    }
  }, [])

  return (
    <div className="shipping-page">
      <SEO title={pageTitle} description={pageDescription} />
      <div className="page-header">
        <h1>ارسال و حمل</h1>
        <p className="page-subtitle">ارسال محصولات ما با دقت و سرعت انجام می‌شود</p>
      </div>

      <div className="shipping-content">
        <div className="content-section">
          <h2>روش‌های ارسال</h2>
          <p>
            ما دو روش ارسال استاندارد و اکسپرس را برای راحتی شما فراهم کرده‌ایم:
          </p>
          <ul className="shipping-options">
            <li>
              <strong>ارسال استاندارد</strong>
              <p>
                مناسب برای سفارش‌هایی که زمان برای شما مهم نیست. معمولاً پس از ۱ تا ۳ روز کاری
                از تایید سفارش آماده ارسال می‌شود.
              </p>
            </li>
            <li>
              <strong>ارسال اکسپرس</strong>
              <p>
                برای مواقعی که سریع‌ترین راه ممکن مورد نیاز است. سفارش‌های اکسپرس در اولویت
                آماده‌سازی قرار می‌گیرند.
              </p>
            </li>
          </ul>
        </div>

        <div className="content-section">
          <h2>هزینه ارسال</h2>
          <p>
            هزینه ارسال بر اساس منطقه و وزن سفارش محاسبه می‌شود. در صفحه سبد خرید قبل از پرداخت，
            هزینه ارسال به شما نمایش داده می‌شود تا به راحتی بتوانید تصمیم بگیرید.
          </p>
          <div className="shipping-notes">
            <p>💡 <strong>نکته:</strong> برای سفارش‌های بالای مبلغ مشخص، هزینه ارسال رایگان ارائه می‌شود.</p>
          </div>
        </div>

        <div className="content-section">
          <h2>زمان ارسال</h2>
          <p>
            زمان ارسال واقعی بستگی به منطقه ساکنی شما و شرایط جاری دارد. به طور معمول:
          </p>
          <ul>
            <li>شهرستان‌های بزرگ: ۱ تا ۳ روز کاری پس از ارسال</li>
            <li>شهرستان‌های کوچک و مناطق دور: ۳ تا ۷ روز کاری پس از ارسال</li>
          </ul>
          <p>
            لطفاً توجه داشته باشید که تاریخ‌های ذکر شده تقریبی هستند و تحت تغییر سرعت پست یا
            شرایط جوی قرار می‌گیرند.
          </p>
        </div>

        <div className="content-section">
          <h2>بسته‌بندی</h2>
          <p>
            سلامت گیاه برای ما بسیار مهم است. تمامی گیاهان به صورت حرفه‌ای بسته‌بندی می‌شوند تا
            در حین حمل و نقل آسیب نبینند. بسته‌بندی ما شامل:
          </p>
          <ul>
            <li>استفاده از جعبه‌های مقاوم</li>
            <li>پوشش گیاه با مواد محافظ</li>
            <li>ثابت کردن گلدان برای جلوگیری از حرکت در حین حمل</li>
          </ul>
        </div>

        <div className="content-section">
          <h2>پیگیری سفارش</h2>
          <p>
            پس از ارسال سفارش، شماره پیگیری به شما ارسال می‌شود تا بتوانید وضعیت دقیق سفارش را
            مانیتور کنید.
          </p>
        </div>

        <div className="content-section">
          <h2>دریافت سفارش</h2>
          <p>
            در زمان دریافت سفارش، توصیه می‌کنیم:
          </p>
          <ul>
            <li>بسته را قبل از باز کردن بررسی کنید</li>
            <li>در صورت مشاهده آسیب ظاهری، آن را اعلام کنید</li>
            <li>گیاه را به محیط جدید آشکار کنید و آب کافی به آن بدهید</li>
          </ul>
        </div>

        <div className="shipping-faq-link">
          <Link to="/faq">سوالات متداول در مورد ارسال</Link>
        </div>
      </div>
    </div>
  )
}
