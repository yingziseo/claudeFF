// Actual packaged Wails/WebView2 UI; no simulated window.go bindings or public credential traces.
import { chromium } from '@playwright/test'
import { spawn, spawnSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
if(process.env.GITHUB_ACTIONS!=='true')throw new Error('Cloud-only native validation')
const raw=process.env.CLAUDEFF_TEST_PROXY
if(!raw)throw new Error('Temporary authorized test proxy secret is required for this validation')
const [host,port,username,...tail]=raw.split(':');const password=tail.join(':')
for(const v of [raw,username,password])console.log(`::add-mask::${v}`)
const dir=path.join(process.env.RUNNER_TEMP,'claudeff-native-ui');fs.mkdirSync(dir,{recursive:true});fs.mkdirSync('native-evidence',{recursive:true})
const env={...process.env,APPDATA:dir,WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS:'--remote-debugging-port=9223 --remote-debugging-address=127.0.0.1'};delete env.CLAUDEFF_TEST_PROXY
let stage='start desktop',browser,processHandle
try{
 processHandle=spawn(path.resolve('build/bin/claudeFF.exe'),[],{env,stdio:'ignore'})
 for(let i=0;i<60;i++){try{browser=await chromium.connectOverCDP('http://127.0.0.1:9223');break}catch{await new Promise(r=>setTimeout(r,500))}}
 if(!browser)throw new Error('debug endpoint unavailable')
 const page=browser.contexts()[0].pages()[0];page.setDefaultTimeout(45000)
 await page.waitForFunction(()=>Boolean(window.go?.main?.App));await page.getByRole('heading',{name:'你的 Claude 工作空间'}).waitFor()
 stage='native clipboard import';const clip=spawnSync('powershell.exe',['-NoProfile','-Command','Set-Clipboard -Value ([Console]::In.ReadToEnd())'],{input:raw,encoding:'utf8',windowsHide:true});if(clip.status!==0)throw new Error('clipboard failed')
 await page.getByRole('button',{name:'全局连接',exact:true}).click();await page.getByRole('button',{name:'粘贴识别',exact:true}).click()
 await page.waitForFunction(()=>document.querySelector('input[name=proxyHost]')?.value!=='')
 const values=await Promise.all(['proxyHost','proxyPort','proxyUsername','proxyPassword'].map(name=>page.locator(`input[name=${name}]`).inputValue()))
 if(values[0]!==host||values[1]!==port||values[2]!==username||values[3]!==password)throw new Error('native clipboard fields incorrect')
 stage='real proxy save and exit detection';await page.getByRole('button',{name:'保存并检测',exact:true}).click();await page.getByRole('heading',{name:'连接已准备',exact:true}).waitFor()
 const snapshot=()=>page.evaluate(async()=>JSON.parse(await window.go.main.App.Invoke('snapshot','{}')))
 let s=await snapshot();if(s.network.observation?.ip!==host||s.network.observation?.countryCode!=='SG'||!(s.network.observation?.latency>0)||!s.hasPassword||s.proxy.password)throw new Error('real proxy observation or secret storage incorrect')
 const detection=s.network.observation
 stage='bundled real fingerprint browser';await page.getByRole('button',{name:'Claude.ai',exact:true}).click();await page.getByRole('button',{name:'打开浏览器',exact:true}).click();await page.getByRole('button',{name:'激活浏览器',exact:true}).waitFor({timeout:90000})
 s=await snapshot();if(!s.kernel||s.browser.phase!=='running'||s.browser.observation?.ip!==host||!s.fingerprint?.userAgent)throw new Error('native browser path not verified')
 await page.getByRole('button',{name:'关闭浏览器',exact:true}).click();await page.getByRole('button',{name:'打开浏览器',exact:true}).waitFor()
 await page.getByRole('button',{name:'概览',exact:true}).click()
 await page.screenshot({path:'native-evidence/windows-home.png',fullPage:true,mask:[page.locator('.connection-strip div>span')]})
 stage='persisted desktop restart';processHandle.kill();await browser.close();browser=undefined
 await new Promise(r=>setTimeout(r,1500));processHandle=spawn(path.resolve('build/bin/claudeFF.exe'),[],{env,stdio:'ignore'})
 for(let i=0;i<60;i++){try{browser=await chromium.connectOverCDP('http://127.0.0.1:9223');break}catch{await new Promise(r=>setTimeout(r,500))}}
 if(!browser)throw new Error('restart debug endpoint unavailable')
 const second=browser.contexts()[0].pages()[0];await second.waitForFunction(()=>Boolean(window.go?.main?.App))
 s=await second.evaluate(async()=>JSON.parse(await window.go.main.App.Invoke('snapshot','{}')))
 if(!s.hasPassword||s.proxy.host!==host||!s.profile)throw new Error('configuration not restored')
 const report={nativeWailsUI:true,nativeClipboard:true,realProxy:true,exitMatchesGateway:true,countryCode:detection.countryCode,city:detection.city,timezone:detection.timezone,latencyMs:detection.latency,bundledKernel:true,browserExitMatches:true,persistentProfile:true,credentialProtected:true,claudeAccountLogin:'not tested'}
 fs.writeFileSync('native-evidence/result.json',JSON.stringify(report,null,2));console.log(JSON.stringify(report))
}catch{console.error(`Native validation failed at: ${stage}. Credential-bearing values and traces are intentionally omitted.`);process.exitCode=1}
finally{if(browser)await browser.close().catch(()=>{});if(processHandle)processHandle.kill();spawnSync('powershell.exe',['-NoProfile','-Command','Set-Clipboard -Value ""'],{stdio:'ignore',windowsHide:true})}
