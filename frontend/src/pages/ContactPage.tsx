import { Link } from "react-router-dom";
import { storeConfig } from "../config";
import SectionHeader from "../components/SectionHeader";
import SEO from "../components/SEO";

type ContactConfig = {
  supportEmail?: string;
  supportPhone?: string;
  supportTelegram?: string;
  supportInstagram?: string;
};

const contactConfig: ContactConfig = {
  supportEmail: import.meta.env.VITE_SUPPORT_EMAIL || undefined,
  supportPhone: import.meta.env.VITE_SUPPORT_PHONE || undefined,
  supportTelegram: import.meta.env.VITE_SUPPORT_TELEGRAM || undefined,
  supportInstagram: import.meta.env.VITE_SUPPORT_INSTAGRAM || undefined,
};

const hasContactInfo = Object.values(contactConfig).some(Boolean);

export default function ContactPage() {
  const pageTitle = "تماس با ما";
  const pageDescription =
    "ارتباط با " + storeConfig.name + " برای سوالات و پشتیبانی";

  return (
    <div className="contact-page">
      <SEO
        title={pageTitle}
        description={pageDescription}
        canonical="/contact"
      />
      <SectionHeader
        title="تماس با ما"
        subtitle="سوالی دارید؟ ما اینجا هستیم تا کمک کنیم"
        align="center"
      />

      <div className="page-content">
        <div className="contact-intro">
          <p>
            در {storeConfig.name} همیشه آماده هستیم تا به سوالات شما پاسخ دهیم.
            از انتخاب محصول تا روش مصرف و خدمات آموزشی، ما در کنار شما هستیم.
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
              راه‌های تماس هنوز توسط فروشگاه ثبت نشده‌اند. لطفاً بعداً دوباره
              بررسی کنید.
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
      </div>
    </div>
  );
}
