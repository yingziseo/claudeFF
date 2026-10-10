import { expect, test } from '@playwright/test'
test('真实管理面板生成、激活、心跳与撤销', async ({ page, request }, info) => {
 test.skip(!process.env.CLAUDEFF_CI_LICENSE, 'Only runs against the compiled cloud license server')
 await page.goto('./#admin'); await page.getByLabel('管理员令牌').fill(process.env.CLAUDEFF_ADMIN_TOKEN!)
 await page.getByRole('button', { name: '进入管理面板' }).click()
 await page.getByLabel('卡密备注').fill('云端完整流程')
 await page.getByRole('button', { name: '生成永久卡密' }).click()
 const card = await page.locator('.generated-key').textContent(); expect(card).toMatch(/^CFF-/)
 await page.getByRole('button', { name: '已保存' }).click()
 const registration = await request.post('http://127.0.0.1:8787/v1/register', { data: { Username: '云测用户', Password: 'ci-test-password', Card: card } }); expect(registration.status()).toBe(200)
 const data = await registration.json() as { token: string }
 const heartbeat = await request.post('http://127.0.0.1:8787/v1/heartbeat', { headers: { Authorization: `Bearer ${data.token}` }, data: { version: '0.2.0', browser: true, code: true } }); expect(heartbeat.status()).toBe(200)
 await page.getByRole('button', { name: '刷新', exact: true }).click(); await page.getByRole('button', { name: '用户', exact: true }).click()
 await expect(page.getByText('云测用户', { exact: true })).toBeVisible(); await expect(page.getByText('最近在线', { exact: true })).toBeVisible()
 await page.screenshot({ path: info.outputPath('admin-users-real.png'), fullPage: true })
 await page.getByRole('button', { name: '卡密', exact: true }).click(); await page.getByRole('button', { name: '撤销', exact: true }).click()
 await page.getByRole('button', { name: '确认撤销' }).click(); await expect(page.getByText('已撤销', { exact: true })).toBeVisible()
 expect((await request.post('http://127.0.0.1:8787/v1/heartbeat', { headers: { Authorization: `Bearer ${data.token}` }, data: { version: '0.2.0', browser: true, code: true } })).status()).toBe(403)
})
