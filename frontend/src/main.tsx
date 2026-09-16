import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { storeConfig } from './config'

// Set the document title from the configured store name. Price display no
// longer goes through a CSS custom property — see src/lib/format.ts.
document.title = storeConfig.name

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
