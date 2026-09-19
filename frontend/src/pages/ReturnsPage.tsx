import { useEffect } from 'react'
import { Link } from 'react-router-dom'
import { storeConfig } from '../config'
import SEO from '../components/SEO'

export default function ReturnsPage() {
  const pageTitle = 'سیاست بازگرداندن'
  const pageDescription = 'سیاست بازگرداندن و بازپرداخت محصولات در ' + storeConfig.name

  useEffect(() => {
    document.title = `${pageTitle} - ${storeConfig.name}`
    const descMeta = document.querySelector('meta[name="description"]')
    if (descMeta) {
      descMeta.setAttribute('content', pageDescription)
    }
  }, [])

  return (
    <div className="returns-page">
      <SEO title={pageTitle} description={pageDescription} />
      <div className="page-header">
        <h1>سیاست بازگرداندن</h1>
        <p className="page-subtitle">
          ما به رضایت مشتریان خود اهمیت ویژه‌ای می‌دهیم
        </p>
      </div>

      <div className="returns-content">
        <div className="content-section">
          <h2>لغو سفارش قبل از ارسال</h2>
          <p>
            در صورتی که سفارش شما هنوز به مرحله ارسال نرسیده، امکان لغو آن وجود دارد. لغو سفارش
            می‌تواند از طریق:
          </p>
          <ul>
            <li>صفحه سبد خرید قبل از پرداخت</li>
            <li>حساب کاربری و بخش «سفارش‌های من»</li>
            <li>تماس با پشتیبانی</li>
          </ul>
          <p>
            پس از لغو موفق، مبلغ سفارش به حساب شما برمی‌گردد. زمان بازگشت مبلغ بستگی به روش
            پرداخت و بانک شما دارد.
          </p>
        </div>

        <div className="content-section">
          <h2>بازگرداندن محصول پس از ارسال</h2>
          <p>
            در حال حاضر، سیستم بازگرداندن خودکار محصولات پس از ارسال در دسترس نیست. اما در صورتی
            که:
          </p>
          <ul>
            <li>محصول با آسیب به دست شما رسید</li>
            <li>محصول با توضیحات آن تفاوت دارد</li>
            <li>مشکل دیگری در محصول مشاهده می‌کنید</li>
          </ul>
          <p>
            لطفاً از طریق <Link to="/contact">تماس با ما</Link> با پشتیبانی تماس بگیرید. ما تلاش
            می‌کنیم بهترین راه حل را پیدا کنیم.
          </p>
        </div>

        <div className="content-section">
          <h2>ملاحظات مهم</h2>
          <div className="returns-notice">
            <p>
              به دلیل طبيعت محصولات ما (گیاهان زنده)، بازگرداندن محصول پس از دریافت دارای محدودیت
              است. لطفاً قبل از خرید:
            </p>
            <ul>
              <li>توضیحات محصول را به دقت مطالعه کنید</li>
              <li>از شرایط نگهداری آگاه شوید</li>
              <li>در صورت نیاز، از آموزش‌های ما استفاده کنید</li>
            </ul>
          </div>
        </div>

        <div className="content-section">
          <h2>ارتباط با پشتیبانی</h2>
          <p>
            برای درخواست بازگرداندن یا لغو سفارش، موارد زیر را آماده کنید:
          </p>
          <ul>
            <li>شماره سفارش</li>
            <li>توضیحات دقیق مشکل یا دلیل لغو</li>
            <li>عکس‌های لازم در صورت وجود آسیب</li>
          </ul>
        </div>

        <div className="returns-contact">
          <h3>نیاز به کمک دارید؟</h3>
          <Link to="/contact" className="btn btn-primary">
            تماس با پشتیبانی
          </Link>
        </div>
      </div>
    </div>
  )
}
