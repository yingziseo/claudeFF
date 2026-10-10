import { defineConfig } from '@playwright/test'
export default defineConfig({
 testDir: './tests', testMatch: 'admin-real.spec.ts', workers: 1, reporter: [['list'], ['html', { outputFolder: 'license-report', open: 'never' }]],
 use: { baseURL: 'http://127.0.0.1:8787/claudeFF/', viewport: { width: 1360, height: 900 }, locale: 'zh-CN' },
 webServer: { command: '../build/license-server -listen 127.0.0.1:8787 -db ../build/ci-license.db -web dist', url: 'http://127.0.0.1:8787/claudeFF/', reuseExistingServer: false },
})
