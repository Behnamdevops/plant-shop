import { useEffect } from 'react'
import { storeConfig } from '../config'
import SEO from '../components/SEO'

export default function PrivacyPage() {
  const pageTitle = 'حریم خصوصی'
  const pageDescription = 'سیاست حفظ حریم خصوصی کاربران در ' + storeConfig.name

  useEffect(() => {
    document.title = `${pageTitle} - ${storeConfig.name}`
    const descMeta = document.querySelector('meta[name="description"]')
    if (descMeta) {
      descMeta.setAttribute('content', pageDescription)
    }
  }, [])

  return (
    <div className="legal-page">
      <SEO title={pageTitle} description={pageDescription} />
      <div className="page-header">
        <h1>حریم خصوصی</h1>
        <p className="page-subtitle">سیاست حفظ حریم خصوصی کاربران</p>
      </div>

      <div className="legal-content">
        <div className="legal-section">
          <h2>۱. مقدمه</h2>
          <p>
            {storeConfig.name} به حریم خصوصی کاربران خود اهمیت ویژه‌ای می‌دهد. این سیاست
            حریم خصوصی توضیح می‌دهد که چگونه اطلاعات شما را جمع‌آوری، استفاده و محافظت
            می‌کنیم.
          </p>
        </div>

        <div className="legal-section">
          <h2>۲. اطلاعاتی که جمع‌آوری می‌کنیم</h2>
          <p>
            هنگام ثبت‌نام یا انجام سفارش، ممکن است اطلاعات زیر را از شما دریافت کنیم:
          </p>
          <ul>
            <li>نام، شماره تماس و ایمیل</li>
            <li>آدرس برای ارسال سفارش</li>
            <li>اطلاعات پرداخت (درگاه پرداخت)</li>
            <li>سایر اطلاعاتی که به داوطلبانه ارائه می‌کنید</li>
          </ul>
        </div>

        <div className="legal-section">
          <h2>۳. استفاده از اطلاعات</h2>
          <p>
            اطلاعات شما برای اهداف زیر استفاده می‌شود:
          </p>
          <ul>
            <li>پردازش سفارش‌ها</li>
            <li>ارتباط با شما در مورد سفارش‌ها</li>
            <li>بهبود خدمات و محصولات</li>
            <li>ارسال اطلاعیه‌های مربوط به خدمات</li>
          </ul>
        </div>

        <div className="legal-section">
          <h2>۴. امنیت اطلاعات</h2>
          <p>
            ما از اقدامات امنیتی مناسب برای محافظت از اطلاعات شما استفاده می‌کنیم. با این حال،
            توجه داشته باشید که هیچ روش انتقال یا ذخیره اطلاعات آنلاین کاملاً امن نیست.
          </p>
        </div>

        <div className="legal-section">
          <h2>۵. اشتراک‌گذاری اطلاعات</h2>
          <p>
            ما اطلاعات شما را با افراد یا سازمان‌های ثالث به منظور ارائه خدمات (مانند
            پردازش سفارش) به اشتراک می‌گذاریم، اما این اشتراک‌گذاری محدود به اطلاعات ضروری
            است.
          </p>
        </div>

        <div className="legal-section">
          <h2>۶. کوکی‌ها</h2>
          <p>
            ما از کوکی‌ها برای بهبود تجربه کاربری استفاده می‌کنیم. با این حال، می‌‌توانید
            تنظیمات مرورگر خود را تغییر دهید تا کوکی‌ها را رد کنید.
          </p>
        </div>

        <div className="legal-section">
          <h2>۷. تغییرات در این سیاست</h2>
          <p>
            ما ممکن است این سیاست حریم خصوصی را به طور دوره‌ای به‌روزرسانی کنیم. تغییرات در
            این صفحه اعلام می‌شود.
          </p>
        </div>

        <div className="legal-section">
          <h2>۸. ارتباط با ما</h2>
          <p>
            اگر سوالی در مورد این سیاست حریم خصوصی دارید، لطفاً با ما تماس بگیرید.
          </p>
        </div>
      </div>
    </div>
  )
}
