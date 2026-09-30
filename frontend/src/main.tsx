import { StrictMode } from 'react'
import { createRoot, hydrateRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { storeConfig } from './config'

// Set the document title from the configured store name. Price display no
// longer goes through a CSS custom property — see src/lib/format.ts.
if (!document.getElementById('public-data')) document.title = storeConfig.name

const root = document.getElementById('root')!
const seed = document.getElementById('public-data')
const initialData = seed?.textContent ? JSON.parse(seed.textContent) : null
const app = (
  <StrictMode>
    <App initialData={initialData} />
  </StrictMode>
)
if (initialData) hydrateRoot(root, app)
else createRoot(root).render(app)
