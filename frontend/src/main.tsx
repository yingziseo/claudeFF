import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import AdminApp from './AdminApp'
import './styles.css'

createRoot(document.getElementById('root')!).render(<StrictMode>{window.location.hash === '#admin' ? <AdminApp /> : <App />}</StrictMode>)
