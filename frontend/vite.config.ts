import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  base: process.env.CLAUDEFF_DESKTOP ? './' : '/claudeFF/',
  plugins: [react(), tailwindcss()],
})
