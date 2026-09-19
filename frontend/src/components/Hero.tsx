import { Link } from 'react-router-dom'

type HeroProps = {
  title?: string
  subtitle?: string
  primaryText?: string
  primaryLink?: string
  secondaryText?: string
  secondaryLink?: string
}

export default function Hero({
  title = 'گیاهان سالم، زندگی سالم‌تر',
  subtitle = 'گیاهات اطراف خانه، محتوای آموزشی و محصولات انتخاب شده برای زندگی بهتر شما',
  primaryText = 'نمایش محصولات',
  primaryLink = '/shop',
  secondaryText = 'مشاهده مقالات',
  secondaryLink = '/articles'
}: HeroProps) {
  return (
    <section className="hero">
      <div className="hero__container">
        <div className="hero__content">
          <h1 className="hero__title">{title}</h1>
          <p className="hero__subtitle">{subtitle}</p>
          <div className="hero__actions">
            <Link to={primaryLink} className="btn btn-primary hero__btn-primary">
              {primaryText}
            </Link>
            <Link to={secondaryLink} className="btn btn-secondary hero__btn-secondary">
              {secondaryText}
            </Link>
          </div>
        </div>
        <div className="hero__visual" aria-hidden="true">
          <div className="hero__visual-decoration">
            <div className="hero__visual-decoration-leaf" />
            <div className="hero__visual-decoration-leaf" />
            <div className="hero__visual-decoration-leaf" />
          </div>
        </div>
      </div>
    </section>
  )
}
