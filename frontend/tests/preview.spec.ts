import { expect, test, type Page } from '@playwright/test'
async function connect(page: Page) {
 await page.getByRole('button',{name:'全局连接',exact:true}).click()
 await page.getByLabel('代理地址',{exact:true}).fill('proxy.example.com')
 await page.getByRole('button',{name:'保存并检测',exact:true}).click()
 await expect(page.getByRole('heading',{name:'连接已准备'})).toBeVisible()
}
test('简洁首页、官方素材和三入口',async({page},info)=>{
 await page.goto('./');await expect(page.getByRole('heading',{name:'你的 Claude 工作空间'})).toBeVisible()
 await expect(page.locator('.product-card')).toHaveCount(2);await expect(page.getByText('正在开发中',{exact:true})).toBeVisible()
 expect(await page.locator('.brand img').getAttribute('src')).toContain('claude-symbol.svg')
 await expect(page.locator('.mini-browser,.mini-terminal')).toHaveCount(0)
 await page.screenshot({path:info.outputPath('workspace-dark.png'),fullPage:true})
})
test('四段式和URL导入，密码遮蔽，商家图片和链接',async({page},info)=>{
 await page.goto('./');await page.getByRole('button',{name:'全局连接',exact:true}).click()
 await page.getByLabel('代理信息识别框').fill('198.51.100.17:7085:demo-user:demo-password')
 await page.getByRole('button',{name:'识别',exact:true}).click()
 await expect(page.getByLabel('代理地址',{exact:true})).toHaveValue('198.51.100.17')
 await expect(page.getByLabel('端口',{exact:true})).toHaveValue('7085')
 await expect(page.getByLabel('代理用户名',{exact:true})).toHaveValue('demo-user')
 await expect(page.getByLabel('代理密码',{exact:true})).toHaveAttribute('type','password')
 await page.getByLabel('代理信息识别框').fill('socks5://hello:p%40ss@proxy.example.com:1080')
 await page.getByRole('button',{name:'识别',exact:true}).click()
 await expect(page.getByLabel('协议',{exact:true})).toHaveValue('socks5')
 await expect(page.getByLabel('代理密码',{exact:true})).toHaveValue('p@ss')
 await expect(page.getByText('中国大陆网络，无法直接使用代理服务，请您自行准备海外网络环境。')).toBeVisible()
 expect(await page.locator('.provider-card a').first().getAttribute('href')).toContain('extendid=30d6N2VjYjI3MzQ4NDUyNTY3OTQxMTg5')
 await expect(page.locator('.provider-card img')).toHaveJSProperty('naturalWidth',600)
 await page.screenshot({path:info.outputPath('connection.png'),fullPage:true,mask:[page.getByLabel('代理用户名',{exact:true}),page.getByLabel('代理密码',{exact:true})]})
})
test('Code可单独安装，无沙箱门槛，浏览器独立创建',async({page},info)=>{
 await page.goto('./');await connect(page);await page.getByRole('button',{name:'Claude Code',exact:true}).click()
 await page.getByRole('button',{name:'安装官方 Code',exact:true}).click()
 await expect(page.getByRole('dialog')).not.toBeVisible();await expect(page.getByRole('button',{name:'启动终端',exact:true})).toBeVisible()
 await page.getByRole('button',{name:'启动终端',exact:true}).click();await expect(page.getByRole('button',{name:'停止终端',exact:true})).toBeVisible()
 await page.screenshot({path:info.outputPath('code-terminal.png'),fullPage:true})
 await page.getByRole('button',{name:'停止终端',exact:true}).click()
 await page.getByRole('button',{name:'Claude.ai',exact:true}).click();await page.getByRole('button',{name:'打开浏览器',exact:true}).click()
 await expect(page.getByText('preview-001',{exact:true})).toBeVisible()
 await expect(page.getByRole('button',{name:'下载浏览器内核'})).toHaveCount(0)
})
test('指纹详情与开发登录仅交互',async({page})=>{
 const writes:string[]=[];page.on('request',r=>{if(r.method()!=='GET')writes.push(r.url())})
 await page.goto('./');await page.getByRole('button',{name:'设置',exact:true}).click();await page.getByRole('button',{name:'浏览器指纹',exact:true}).click()
 await expect(page.getByRole('dialog')).toContainText('Canvas 指纹样本');await page.keyboard.press('Escape')
 await page.getByRole('button',{name:'开发体验',exact:true}).click();await page.getByRole('button',{name:'注册',exact:true}).click()
 await page.getByLabel('用户名',{exact:true}).fill('任意用户');await page.getByRole('button',{name:'进入工作空间'}).click();expect(writes).toEqual([])
})
test('明暗主题持久化和375px无横向溢出',async({page},info)=>{
 await page.goto('./');await page.getByRole('button',{name:'切换浅色主题'}).click();await page.screenshot({path:info.outputPath('workspace-light.png'),fullPage:true})
 await page.reload();await expect(page.locator('html')).toHaveAttribute('data-theme','light')
 await page.setViewportSize({width:375,height:812});await page.screenshot({path:info.outputPath('mobile.png'),fullPage:true})
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
 await page.getByRole('button',{name:'全局连接',exact:true}).click();await page.screenshot({path:info.outputPath('mobile-connection.png'),fullPage:true})
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
})
test('800px高度主按钮可见，键盘和减少动画',async({page})=>{
 await page.setViewportSize({width:1280,height:800});await page.emulateMedia({reducedMotion:'reduce'});await page.goto('./')
 for(const name of ['打开 Claude.ai','安装并设置']){const b=await page.getByRole('button',{name,exact:true}).boundingBox();expect(b!.y+b!.height).toBeLessThan(800)}
 await page.keyboard.press('Tab');await expect(page.getByRole('link',{name:'跳到主要内容'})).toBeFocused();await page.keyboard.press('Enter');await expect(page.locator('#workspace-main')).toBeFocused()
})
test('初始化错误可恢复',async({page})=>{
 await page.addInitScript(()=>{window.go={main:{App:{Invoke:async()=>{throw new Error('测试：目录不可写')}}}}})
 await page.goto('./');await expect(page.getByRole('alert')).toContainText('目录不可写');await expect(page.getByRole('button',{name:'重试',exact:true})).toBeVisible()
})
for(const missing of [false,true])test(`原生桥接内核${missing?'缺失时提示重新解压':'随包直接启动'}，不触发下载`,async({page})=>{
 await page.addInitScript(missing=>{
  const calls:string[]=[];Object.defineProperty(window,'testCalls',{value:calls})
  const s={proxy:{protocol:'http',host:'fixture.example.com',port:8080,username:''},hasPassword:false,revision:1,browser:{phase:'idle',message:'尚未启动'},code:{phase:'idle',message:'尚未安装'},network:{phase:'ready',message:'测试',observation:{ip:'203.0.113.12',at:new Date().toISOString()}},kernel:!missing,native:true,dataDir:'测试目录'}
  window.go={main:{App:{Invoke:async(action:string)=>{if(action!=='snapshot')calls.push(action);if(action==='browser.start')s.browser={phase:'running',message:'已启动'};return JSON.stringify(s)}}}}
 },missing)
 await page.goto('./');await page.getByRole('button',{name:'打开 Claude.ai',exact:true}).click()
 if(missing)await expect(page.getByRole('alert')).toContainText('重新解压完整 ZIP');else await expect(page.getByRole('button',{name:'激活浏览器',exact:true})).toBeVisible()
 expect(await page.evaluate(()=>(window as unknown as{testCalls:string[]}).testCalls)).toEqual(missing?[]:['browser.start'])
})
test('管理面板预览生成和撤销',async({page})=>{
 await page.goto('./#admin');await page.getByRole('button',{name:'进入管理面板'}).click();await page.getByLabel('卡密备注').fill('体验');await page.getByRole('button',{name:'生成永久卡密'}).click();await page.getByRole('button',{name:'已保存'}).click()
 await page.getByLabel('搜索卡密或用户').fill('体验');await page.getByRole('button',{name:'撤销',exact:true}).first().click();await page.getByRole('button',{name:'确认撤销'}).click()
 await expect(page.getByText('已撤销',{exact:true}).first()).toBeVisible()
})
