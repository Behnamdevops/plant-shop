import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { storeConfig } from './config'

// Expose the configured currency symbol to CSS (see .price::before in
// App.css) and set the document title from the configured store name.
document.documentElement.style.setProperty(
  '--currency-symbol',
  `'${storeConfig.currencySymbol}'`,
)
document.title = storeConfig.name

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
