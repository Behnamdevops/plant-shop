import { useState } from 'react'

type ProductImageProps = {
  src: string | null
  alt: string
  className?: string
  placeholderClassName?: string
}

// ProductImage renders a product's image with a graceful fallback: shows
// the leaf placeholder when there is no image_url at all, and falls back
// to the same placeholder if the URL fails to load (e.g. a broken external
// link, or a locally uploaded image that was removed).
export default function ProductImage({ src, alt, className, placeholderClassName }: ProductImageProps) {
  const [failed, setFailed] = useState(false)

  if (!src || failed) {
    return (
      <span className={placeholderClassName} aria-hidden="true">
        🌱
      </span>
    )
  }

  return <img src={src} alt={alt} className={className} onError={() => setFailed(true)} />
}
