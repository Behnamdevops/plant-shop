type StoreBenefitCardProps = {
  title: string
  description: string
  icon?: string
  className?: string
}

export default function StoreBenefitCard({
  title,
  description,
  icon = '🌿',
  className = ''
}: StoreBenefitCardProps) {
  return (
    <div className={`store-benefit-card ${className}`}>
      <div className="store-benefit-card__icon" aria-hidden="true">
        {icon}
      </div>
      <h3 className="store-benefit-card__title">{title}</h3>
      <p className="store-benefit-card__description">{description}</p>
    </div>
  )
}
