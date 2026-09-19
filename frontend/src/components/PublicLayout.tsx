import type { ReactNode } from 'react'
import Footer from './Footer'
import Header from './Header'

type PublicLayoutProps = {
  children: ReactNode
}

export default function PublicLayout({ children }: PublicLayoutProps) {
  return (
    <>
      <Header isPublic={true} />
      <main className="public-layout">{children}</main>
      <Footer />
    </>
  )
}
