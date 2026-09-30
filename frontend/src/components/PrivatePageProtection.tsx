import { useEffect } from 'react'
import { useLocation } from 'react-router-dom'

const PRIVATE_ROUTES = [
  '/admin',
  '/account',
  '/cart',
  '/checkout',
  '/orders',
  '/payment/result',
  '/login',
  '/register',
  '/wishlist',
  '/forgot-password',
  '/reset-password',
]

function isPrivateRoute(pathname: string): boolean {
  return PRIVATE_ROUTES.some((route) => pathname.startsWith(route))
}

export default function PrivatePageProtection() {
  const location = useLocation()

  useEffect(() => {
    if (isPrivateRoute(location.pathname)) {
      let robotsMeta = document.querySelector('meta[name="robots"]')
      if (!robotsMeta) {
        robotsMeta = document.createElement('meta')
        robotsMeta.setAttribute('name', 'robots')
        document.head.appendChild(robotsMeta)
      }
      robotsMeta.setAttribute('content', 'noindex, nofollow')
    }
  }, [location.pathname])

  return null
}
