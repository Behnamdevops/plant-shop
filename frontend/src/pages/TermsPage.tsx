import { useEffect } from 'react'
import { storeConfig } from '../config'
import SEO from '../components/SEO'

export default function TermsPage() {
  const pageTitle = 'شرایط و قوانین'
  const pageDescription = 'شرایط و قوانین استفاده از خدمات ' + storeConfig.name

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
        <h1>شرایط و قوانین</h1>
        <p className="page-subtitle">شرایط استفاده از خدمات</p>
      </div>

      <div className="legal-content">
        <div className="legal-section">
          <h2>۱. پذیرش شرایط</h2>
          <p>
            با دسترسی به وب‌سایت {storeConfig.name}، شما این شرایط و قوانین را پذیرفته می‌‌باشید.
          </p>
        </div>

        <div className="legal-section">
          <h2>۲. توضیحات محصول</h2>
          <p>
            ما تلاش می‌کنیم تا توضیحات محصولات را دقیق ارائه دهیم. با این حال، تغییرات محصولات
            از سوی تامین‌کنندگان ممکن است اتفاق بیفتد. ما مسئولیتی برای خطاهای تصادفی نداریم.
          </p>
        </div>

        <div className="legal-section">
          <h2>۳. قیمت‌ها</h2>
          <p>
            قیمت‌ها ممکن است به دلیل تغییرات بازار تغییر کنند. ما قیمت‌ها را به روز نگه می‌داریم
            اما تضمینی برای دقت مطلق نداریم.
          </p>
        </div>

        <div className="legal-section">
          <h2>۴. سفارش‌ها</h2>
          <p>
            با ارسال سفارش، شما تایید می‌کنید که اطلاعات ارائه شده صحیح است و مسئولیت پرداخت
            را بر عهده دارید.
          </p>
        </div>

        <div className="legal-section">
          <h2>۵. مالکیت معنوی</h2>
          <p>
            تمامی محتوای وب‌سایت شامل متون، تصاویر و طراحی، متعلق به {storeConfig.name} است
            و بدون اجازه کپی نمی‌شود.
          </p>
        </div>

        <div className="legal-section">
          <h2>۶. محدودیت مسئولیت</h2>
          <p>
            {storeConfig.name} مسئولیتی برای آسیب‌های ناشی از استفاده از محصولات یا خدمات ندارد.
          </p>
        </div>

        <div className="legal-section">
          <h2>۷. قانون اجرایی</h2>
          <p>
            این شرایط بر اساس قوانین جمهوری اسلامی ایران اجرا می‌شود.
          </p>
        </div>

        <div className="legal-section">
          <h2>۸. ارتباط با ما</h2>
          <p>
            در صورت بروز هرگونه اختلاف، ابتدا تلاش برای حل و فصل دو طرفه انجام می‌شود.
          </p>
        </div>
      </div>
    </div>
  )
}
