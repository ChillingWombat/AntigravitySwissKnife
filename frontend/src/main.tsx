import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { initTheme, normalizeThemeMode } from './theme'
import { api } from './api'

// Bootstrap the UI theme before React mounts. The inline script in index.html
// already applied the stored theme; this reconciles it with the persisted
// backend setting (localStorage remains the fallback) and follows OS changes.
void initTheme(async () => normalizeThemeMode((await api.getTheme())?.theme))

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
