import { Link } from "react-router-dom";
import Icon from "./Icon";
type HeroProps = {
  title?: string;
  subtitle?: string;
  primaryText?: string;
  primaryLink?: string;
  secondaryText?: string;
  secondaryLink?: string;
};
export default function Hero({
  title = "هر چیزی که گیاهت برای رشد بهتر نیاز دارد",
  subtitle = "کود، خاک، ابزار و آموزش برای مراقبت آگاهانه از گیاهان؛ انتخاب محصول با اطلاعات روشن.",
  primaryText = "مشاهدهٔ محصولات",
  primaryLink = "/shop",
  secondaryText = "شروع یادگیری",
  secondaryLink = "/blog",
}: HeroProps) {
  return (
    <section className="hero hero-editorial">
      <div className="hero__container">
        <div className="hero__content">
          <span className="eyebrow">
            <span /> زندگی، کمی سبزتر
          </span>
          <h1 className="hero__title">{title}</h1>
          <p className="hero__subtitle">{subtitle}</p>
          <div className="hero__actions">
            <Link to={primaryLink} className="btn btn-primary">
              {primaryText}
              <Icon name="arrow" size={19} />
            </Link>
            <Link to={secondaryLink} className="hero-text-link">
              {secondaryText}
            </Link>
          </div>
          <div className="hero-note">
            <Icon name="leaf" />
            <span>از شناخت نیاز گیاه تا انتخاب محصول مناسب، قدم‌به‌قدم.</span>
          </div>
        </div>
        <div className="hero__visual">
          <img
            src="/care-products.svg"
            alt="تصویرسازی محصولات مراقبت از گیاه؛ کود، خاک، ابزار و آموزش"
            fetchPriority="high"
          />
          <div className="hero-caption">
            <Icon name="sun" />
            <div>
              <strong>انتخاب آگاهانه</strong>
              <span>مشخصات محصول، روش مصرف و آموزش</span>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
