import { Link } from 'react-router-dom'

type SectionHeaderProps = {
  title: string
  subtitle?: string
  align?: 'left' | 'center'
  linkTo?: string
  linkText?: string
}

export default function SectionHeader({
  title,
  subtitle,
  align = 'left',
  linkTo,
  linkText
}: SectionHeaderProps) {
  return (
    <div className={`section-header section-header--${align}`}>
      <h2 className="section-header__title">{title}</h2>
      {subtitle && <p className="section-header__subtitle">{subtitle}</p>}
      {linkTo && linkText && (
        <Link to={linkTo} className="section-header__link">
          {linkText} →
        </Link>
      )}
    </div>
  )
}
