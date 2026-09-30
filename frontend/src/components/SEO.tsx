import { useEffect } from 'react'
import { storeConfig } from '../config'

interface SEOProps {
  title: string
  description?: string
  canonical?: string
  noindex?: boolean
  // Open Graph props
  ogTitle?: string
  ogDescription?: string
  ogImage?: string
  ogType?: string
  ogUrl?: string
  // Twitter props
  twitterCard?: string
  twitterTitle?: string
  twitterDescription?: string
  twitterImage?: string
}

const DEFAULT_OG_IMAGE = '/og-default.png'
const CANONICAL_ORIGIN = storeConfig.publicOrigin

export default function SEO({
  title,
  description,
  canonical,
  noindex = false,
  ogTitle,
  ogDescription,
  ogImage,
  ogType = 'website',
  ogUrl,
  twitterCard = 'summary_large_image',
  twitterTitle,
  twitterDescription,
  twitterImage,
}: SEOProps) {
  function setOgTag(property: string, content: string) {
    let meta = document.querySelector(`meta[property="${property}"]`)
    if (!meta) {
      meta = document.createElement('meta')
      meta.setAttribute('property', property)
      document.head.appendChild(meta)
    }
    meta.setAttribute('content', content)
  }

  function setTwitterTag(name: string, content: string) {
    let meta = document.querySelector(`meta[name="${name}"]`)
    if (!meta) {
      meta = document.createElement('meta')
      meta.setAttribute('name', name)
      document.head.appendChild(meta)
    }
    meta.setAttribute('content', content)
  }

  useEffect(() => {
    document.title = `${title} - ${storeConfig.name}`

    // Description meta
    let descMeta = document.querySelector('meta[name="description"]')
    if (!descMeta) {
      descMeta = document.createElement('meta')
      descMeta.setAttribute('name', 'description')
      document.head.appendChild(descMeta)
    }
    descMeta.setAttribute('content', description || title)

    // Canonical URL - use absolute URL
    let linkMeta = document.querySelector('link[rel="canonical"]')
    if (!linkMeta) {
      linkMeta = document.createElement('link')
      linkMeta.setAttribute('rel', 'canonical')
      document.head.appendChild(linkMeta)
    }
    linkMeta.setAttribute(
      'href',
      new URL(canonical || window.location.pathname, CANONICAL_ORIGIN).href,
    )

    // Robots meta
    let robotsMeta = document.querySelector('meta[name="robots"]')
    if (!robotsMeta) {
      robotsMeta = document.createElement('meta')
      robotsMeta.setAttribute('name', 'robots')
      document.head.appendChild(robotsMeta)
    }
    robotsMeta.setAttribute(
      'content',
      noindex ? 'noindex, nofollow' : 'index, follow',
    )

    // Open Graph tags - use absolute URLs
    setOgTag('og:title', ogTitle || title)
    setOgTag('og:description', ogDescription || description || title)
    setOgTag('og:type', ogType)
    setOgTag(
      'og:url',
      ogUrl ||
        new URL(canonical || window.location.pathname, CANONICAL_ORIGIN).href,
    )
    setOgTag(
      'og:image',
      new URL(ogImage || DEFAULT_OG_IMAGE, CANONICAL_ORIGIN).href,
    )
    setOgTag('og:site_name', storeConfig.name)

    // Twitter Card tags
    setTwitterTag('twitter:card', twitterCard)
    setTwitterTag('twitter:title', twitterTitle || title)
    setTwitterTag(
      'twitter:description',
      twitterDescription || description || title,
    )
    setTwitterTag(
      'twitter:image',
      new URL(twitterImage || ogImage || DEFAULT_OG_IMAGE, CANONICAL_ORIGIN)
        .href,
    )
  }, [
    title,
    description,
    canonical,
    noindex,
    ogImage,
    ogTitle,
    ogDescription,
    ogType,
    ogUrl,
    twitterCard,
    twitterTitle,
    twitterDescription,
    twitterImage,
  ])

  return null
}
