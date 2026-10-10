// Actual packaged Wails/WebView2 UI; no simulated window.go bindings or public credential traces.
import { chromium } from '@playwright/test'
import { spawn, spawnSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import http from 'node:http'
import net from 'node:net'
import crypto from 'node:crypto'
if(process.env.GITHUB_ACTIONS!=='true')throw new Error('Cloud-only native validation')
const customerProxy=Boolean(process.env.CLAUDEFF_TEST_PROXY)
let raw=process.env.CLAUDEFF_TEST_PROXY, fixture
if(!raw){
 const auth='native-ci:'+crypto.randomBytes(18).toString('hex')
 fixture=http.createServer((_,res)=>{res.writeHead(405);res.end()})
 fixture.on('connect',(req,socket,head)=>{
  if(req.headers['proxy-authorization']!=='Basic '+Buffer.from(auth).toString('base64')){socket.end('HTTP/1.1 407 Proxy Authentication Required\r\n\r\n');return}
  const dest=new URL('http://'+req.url);const upstream=net.connect(Number(dest.port),dest.hostname)
  upstream.on('error',()=>socket.destroy());socket.on('error',()=>upstream.destroy())
  upstream.on('connect',()=>{socket.write('HTTP/1.1 200 Connection Established\r\n\r\n');if(head.length)upstream.write(head);socket.pipe(upstream);upstream.pipe(socket)})
  socket.on('close',()=>upstream.destroy());upstream.on('close',()=>socket.destroy())
 })
 await new Promise(resolve=>fixture.listen(0,'127.0.0.1',resolve))
 raw=`127.0.0.1:${fixture.address().port}:${auth}`
}
const [host,port,username,...tail]=raw.split(':');const password=tail.join(':')
for(const v of [raw,username,password])console.log(`::add-mask::${v}`)
const dir=path.join(process.env.RUNNER_TEMP,'claudeff-native-ui');fs.mkdirSync(dir,{recursive:true});fs.mkdirSync('native-evidence',{recursive:true})
const env={...process.env,APPDATA:dir,WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS:'--remote-debugging-port=9223 --remote-debugging-address=127.0.0.1'};delete env.CLAUDEFF_TEST_PROXY
const debugPolicy='HKLM:\\Software\\Policies\\Microsoft\\Edge\\WebView2\\AdditionalBrowserArguments'
const policy=spawnSync('powershell.exe',['-NoProfile','-Command',`New-Item -Path '${debugPolicy}' -Force | Out-Null; New-ItemProperty -Path '${debugPolicy}' -Name 'claudeFF.exe' -Value '--remote-debugging-port=9223 --remote-debugging-address=127.0.0.1' -PropertyType String -Force | Out-Null`],{encoding:'utf8',windowsHide:true});if(policy.status!==0)throw new Error('Cloud debugging policy setup failed')
let stage='start desktop',browser,processHandle,page,stderr='',processExit=null,lastConnectError=''
const clean=v=>{let s=String(v);for(const secret of [raw,username,password])if(secret)s=s.split(secret).join('[redacted]');return s.slice(0,12000)}
const launch=()=>{const p=spawn(path.resolve('build/bin/claudeFF.exe'),[],{env,stdio:['ignore','ignore','pipe']});p.stderr.on('data',b=>{stderr=(stderr+b.toString()).slice(-16000)});p.on('exit',code=>{processExit=code});return p}
try{
 processHandle=launch()
 for(let i=0;i<60;i++){try{browser=await chromium.connectOverCDP('http://127.0.0.1:9223');break}catch(e){lastConnectError=clean(e.message);await new Promise(r=>setTimeout(r,500))}}
 if(!browser)throw new Error('debug endpoint unavailable')
 page=browser.contexts()[0].pages()[0];page.setDefaultTimeout(45000)
 await page.waitForFunction(()=>Boolean(window.go?.main?.App));await page.getByRole('heading',{name:'你的 Claude 工作空间'}).waitFor()
 stage='native clipboard import';const clip=spawnSync('powershell.exe',['-NoProfile','-Command','Set-Clipboard -Value ([Console]::In.ReadToEnd())'],{input:raw,encoding:'utf8',windowsHide:true});if(clip.status!==0)throw new Error('clipboard failed')
 await page.getByRole('button',{name:'全局连接',exact:true}).click();await page.getByRole('button',{name:'粘贴识别',exact:true}).click()
 await page.waitForFunction(()=>document.querySelector('input[name=proxyHost]')?.value!=='')
 const values=await Promise.all(['proxyHost','proxyPort','proxyUsername','proxyPassword'].map(name=>page.locator(`input[name=${name}]`).inputValue()))
 if(values[0]!==host||values[1]!==port||values[2]!==username||values[3]!==password)throw new Error('native clipboard fields incorrect')
 stage='real proxy save and exit detection';await page.getByRole('button',{name:'保存并检测',exact:true}).click();await page.getByRole('heading',{name:'连接已准备',exact:true}).waitFor()
 const snapshot=()=>page.evaluate(async()=>JSON.parse(await window.go.main.App.Invoke('snapshot','{}')))
 let s=await snapshot();if(!net.isIP(s.network.observation?.ip||'')||(customerProxy&&(s.network.observation.ip!==host||s.network.observation.countryCode!=='SG'))||!(s.network.observation?.latency>0)||!s.hasPassword||s.proxy.password)throw new Error('real proxy observation or secret storage incorrect')
 const detection=s.network.observation
 stage='bundled real fingerprint browser';await page.getByRole('button',{name:'Claude.ai',exact:true}).click();await page.getByRole('button',{name:'打开浏览器',exact:true}).click();await page.getByRole('button',{name:'激活浏览器',exact:true}).waitFor({timeout:90000})
 s=await snapshot();if(!s.kernel||s.browser.phase!=='running'||s.browser.observation?.ip!==detection.ip||!s.fingerprint?.userAgent)throw new Error('native browser path not verified')
 await page.getByRole('button',{name:'关闭浏览器',exact:true}).click();await page.getByRole('button',{name:'打开浏览器',exact:true}).waitFor()
 await page.getByRole('button',{name:'概览',exact:true}).click()
 await page.screenshot({path:'native-evidence/windows-home.png',fullPage:true,mask:[page.locator('.connection-strip div>span')]})
 stage='persisted desktop restart';processHandle.kill();await browser.close();browser=undefined
 await new Promise(r=>setTimeout(r,1500));processHandle=launch()
 for(let i=0;i<60;i++){try{browser=await chromium.connectOverCDP('http://127.0.0.1:9223');break}catch(e){lastConnectError=clean(e.message);await new Promise(r=>setTimeout(r,500))}}
 if(!browser)throw new Error('restart debug endpoint unavailable')
 const second=browser.contexts()[0].pages()[0];await second.waitForFunction(()=>Boolean(window.go?.main?.App))
 s=await second.evaluate(async()=>JSON.parse(await window.go.main.App.Invoke('snapshot','{}')))
 if(!s.hasPassword||s.proxy.host!==host||!s.profile)throw new Error('configuration not restored')
 const report={nativeWailsUI:true,nativeClipboard:true,realProxy:customerProxy,authenticatedFixture:!customerProxy,exitMatchesGateway:customerProxy?true:undefined,exitMatchesProbe:true,countryCode:detection.countryCode,city:detection.city,timezone:detection.timezone,latencyMs:detection.latency,bundledKernel:true,browserExitMatches:true,persistentProfile:true,credentialProtected:true,claudeAccountLogin:'not tested'}
 fs.writeFileSync('native-evidence/result.json',JSON.stringify(report,null,2));console.log(JSON.stringify(report))
}catch(e){
 const failure={stage,error:clean(e.message),processExit,stderr:clean(stderr),connectionError:lastConnectError}
 if(stage==='start desktop'&&processHandle){const diag=spawnSync('powershell.exe',['-NoProfile','-File','frontend/tests/desktop-diagnose.ps1','-TaskProcessId',String(processHandle.pid)],{encoding:'utf8',windowsHide:true,timeout:15000});failure.desktop=clean(diag.stdout||diag.stderr||'')}
 if(page){failure.alerts=clean((await page.locator('[role=alert]').allTextContents().catch(()=>[])).join(' '));await page.screenshot({path:'native-evidence/failure.png',fullPage:true,mask:[page.locator('input'),page.locator('.property dd'),page.locator('.connection-strip div>span')]}).catch(()=>{})}
 fs.writeFileSync('native-evidence/failure.json',JSON.stringify(failure,null,2));console.error(JSON.stringify(failure));process.exitCode=1
}
finally{if(browser)await browser.close().catch(()=>{});if(processHandle)processHandle.kill();spawnSync('powershell.exe',['-NoProfile','-Command','Set-Clipboard -Value ""'],{stdio:'ignore',windowsHide:true});if(fixture)fixture.close();spawnSync('powershell.exe',['-NoProfile','-Command',`Remove-ItemProperty -Path '${debugPolicy}' -Name 'claudeFF.exe' -ErrorAction SilentlyContinue`],{stdio:'ignore',windowsHide:true})}
