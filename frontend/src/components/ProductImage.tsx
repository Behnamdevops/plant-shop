import { useState } from 'react'
import Icon from './Icon'
type Props = {
  src: string | null
  alt: string
  className?: string
  placeholderClassName?: string
  priority?: boolean
}
export default function ProductImage({
  src,
  alt,
  className,
  placeholderClassName,
  priority = false,
}: Props) {
  const [failedSrc, setFailedSrc] = useState<string | null>(null)
  if (!src || failedSrc === src)
    return (
      <span
        className={`${placeholderClassName || ''} image-placeholder`}
        role="img"
        aria-label={`${alt}؛ تصویر ثبت نشده`}
      >
        <Icon name="leaf" size={52} />
        <small>تصویر محصول</small>
      </span>
    )
  return (
    <img
      src={src}
      alt={alt}
      className={className}
      loading={priority ? 'eager' : 'lazy'}
      decoding="async"
      fetchPriority={priority ? 'high' : 'auto'}
      onError={() => setFailedSrc(src)}
    />
  )
}
