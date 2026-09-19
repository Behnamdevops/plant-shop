import { useEffect } from 'react'
import { useLocation } from 'react-router-dom'
import { storeConfig } from '../config'

interface SEOProps {
  title: string
  description?: string
}

export default function SEO({ title, description }: SEOProps) {
  const location = useLocation()

  useEffect(() => {
    document.title = `${title} - ${storeConfig.name}`
    
    let descMeta = document.querySelector('meta[name="description"]')
    if (!descMeta) {
      descMeta = document.createElement('meta')
      descMeta.setAttribute('name', 'description')
      document.head.appendChild(descMeta)
    }
    
    descMeta.setAttribute('content', description || title)
    
    // Update canonical URL
    let linkMeta = document.querySelector('link[rel="canonical"]')
    if (!linkMeta) {
      linkMeta = document.createElement('link')
      linkMeta.setAttribute('rel', 'canonical')
      document.head.appendChild(linkMeta)
    }
    linkMeta.setAttribute('href', window.location.href)
  }, [title, description, location.pathname])

  return null
}
