import { useEffect } from 'react'
import { Link } from 'react-router-dom'
import { storeConfig } from '../config'
import SectionHeader from '../components/SectionHeader'
import SEO from '../components/SEO'

type ContactConfig = {
  supportEmail?: string
  supportPhone?: string
  supportTelegram?: string
  supportInstagram?: string
}

const contactConfig: ContactConfig = {
  supportEmail: import.meta.env.VITE_SUPPORT_EMAIL || undefined,
  supportPhone: import.meta.env.VITE_SUPPORT_PHONE || undefined,
  supportTelegram: import.meta.env.VITE_SUPPORT_TELEGRAM || undefined,
  supportInstagram: import.meta.env.VITE_SUPPORT_INSTAGRAM || undefined,
}

export default function ContactPage() {
  const pageTitle = 'تماس با ما'
  const pageDescription = 'ارتباط با ' + storeConfig.name + ' برای سوالات و پشتیبانی'

  useEffect(() => {
    document.title = `${pageTitle} - ${storeConfig.name}`
    const descMeta = document.querySelector('meta[name="description"]')
    if (descMeta) {
      descMeta.setAttribute('content', pageDescription)
    }
  }, [])

  const hasContactInfo = contactConfig.supportEmail || contactConfig.supportPhone

  return (
    <div className="contact-page">
      <SEO title={pageTitle} description={pageDescription} />
      <SectionHeader
        title="تماس با ما"
        subtitle="سوالی دارید؟ ما اینجا هستیم تا کمک کنیم"
        align="center"
      />

      <div className="page-content">
        <div className="contact-intro">
          <p>
            در {storeConfig.name} همیشه آماده هستیم تا به سوالات شما پاسخ دهیم.
            از انتخاب گیاه تا مشکلات نگهداری، ما در کنار شما هستیم.
          </p>
        </div>

        {hasContactInfo ? (
          <div className="contact-info">
            <div className="contact-method">
              <h3>پشتیبانی آنلاین</h3>
              <div className="contact-method__details">
                {contactConfig.supportEmail && (
                  <p>
                    <span className="contact-label">ایمیل:</span>
                    <a
                      href={`mailto:${contactConfig.supportEmail}`}
                      className="contact-value"
                    >
                      {contactConfig.supportEmail}
                    </a>
                  </p>
                )}
                {contactConfig.supportPhone && (
                  <p>
                    <span className="contact-label">تلفن:</span>
                    <a
                      href={`tel:${contactConfig.supportPhone}`}
                      className="contact-value"
                    >
                      {contactConfig.supportPhone}
                    </a>
                  </p>
                )}
              </div>
            </div>

            <div className="contact-method">
              <h3>شبکه‌های اجتماعی</h3>
              <div className="contact-method__details">
                {contactConfig.supportTelegram && (
                  <p>
                    <span className="contact-label">تلگرام:</span>
                    <a
                      href={`https://t.me/${contactConfig.supportTelegram}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="contact-value"
                    >
                      @{contactConfig.supportTelegram}
                    </a>
                  </p>
                )}
                {contactConfig.supportInstagram && (
                  <p>
                    <span className="contact-label">اینستاگرام:</span>
                    <a
                      href={`https://instagram.com/${contactConfig.supportInstagram}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="contact-value"
                    >
                      @{contactConfig.supportInstagram}
                    </a>
                  </p>
                )}
              </div>
            </div>
          </div>
        ) : (
          <div className="contact-placeholder">
            <p>
              برای تماس با ما از طریق فرم زیر اقدام کنید.
              پشتیبانی ما در اسرع وقت به شما پاسخ خواهد داد.
            </p>
          </div>
        )}

        <div className="contact-links">
          <h3>لینک‌های مفید</h3>
          <div className="contact-links__list">
            <Link to="/faq">سوالات متداول</Link>
            <Link to="/shipping">ارسال و حمل</Link>
            <Link to="/returns">سیاست بازگرداندن</Link>
          </div>
        </div>

        {hasContactInfo ? (
          <div className="contact-form-note">
            <p>
              در حال حاضر فرم آنلاین در دسترس نیست. لطفاً از روش‌های ارتباطی بالا استفاده کنید.
            </p>
          </div>
        ) : (
          <div className="contact-form">
            <form className="contact-form__fields">
              <div className="form-group">
                <label htmlFor="name">نام و نام خانوادگی</label>
                <input
                  type="text"
                  id="name"
                  className="form-input"
                  placeholder="نام خود را وارد کنید"
                  disabled
                />
              </div>
              <div className="form-group">
                <label htmlFor="email">ایمیل</label>
                <input
                  type="email"
                  id="email"
                  className="form-input"
                  placeholder="ایمیل خود را وارد کنید"
                  disabled
                />
              </div>
              <div className="form-group">
                <label htmlFor="message">پیام شما</label>
                <textarea
                  id="message"
                  className="form-input form-textarea"
                  placeholder="پیام خود را بنویسید"
                  rows={5}
                  disabled
                />
              </div>
              <div className="form-note">
                <p>
                  این فرم در حال حاضر فعال نیست. لطفاً از روش‌های ارتباطی بالا استفاده کنید.
                </p>
              </div>
            </form>
          </div>
        )}

        <div className="contact-availability">
          <h3>ساعات پشتیبانی</h3>
          <ul className="availability-list">
            <li>شنبه تا چهارشنبه: ۹:۰۰ تا ۱۸:۰۰</li>
            <li>پنج‌شنبه: ۹:۰۰ تا ۱۴:۰۰</li>
            <li>جمعه: تعطیل</li>
          </ul>
        </div>
      </div>
    </div>
  )
}
