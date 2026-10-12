// Cloud-only acceptance of the actual, unmodified upstream Windows application.
import { chromium } from '../../frontend/node_modules/@playwright/test/index.mjs';
import { spawn, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import net from 'node:net';
import crypto from 'node:crypto';
import assert from 'node:assert/strict';
if(process.env.GITHUB_ACTIONS !== 'true' || process.platform !== 'win32') throw Error('Windows Actions only');
const delay=ms=>new Promise(r=>setTimeout(r,ms));
const output=path.resolve('donut-evidence');fs.mkdirSync(output,{recursive:true});
const exe=path.resolve(process.env.DONUT_EXE);
const auth='baseline:'+crypto.randomBytes(20).toString('hex');
const authorization='Basic '+Buffer.from(auth).toString('base64');
const visits=[];let rejectProbes=false,authenticatedConnections=0;
const sockets=new Set();
function track(s){sockets.add(s);s.on('close',()=>sockets.delete(s));s.on('error',()=>{});return s}
const fixture=http.createServer((req,res)=>{
 const u=new URL(req.url,'http://claudeff-ci.test');
 if(u.pathname==='/receipt'){
  visits.push(Object.fromEntries(u.searchParams));res.end('ok');return;
 }
 res.writeHead(200,{'Content-Type':'text/html; charset=utf-8','Cache-Control':'no-store','Set-Cookie':'baseline=persisted; Max-Age=3600; Path=/'});
 res.end(`<!doctype html><title>ClaudeFF baseline ${u.pathname}</title><h1>Browser session fixture</h1><script>
 const previous=localStorage.getItem('baseline');localStorage.setItem('baseline','persisted');
 fetch('/receipt?path='+encodeURIComponent(location.pathname)+'&previous='+previous+'&cookie='+encodeURIComponent(document.cookie)+'&ua='+encodeURIComponent(navigator.userAgent));
 </script>`);
});
await new Promise(r=>fixture.listen(0,'127.0.0.1',r));
const proxy=http.createServer((req,res)=>{
 if(req.headers['proxy-authorization']!==authorization){res.writeHead(407,{'Proxy-Authenticate':'Basic realm="baseline"'});res.end();return}
 authenticatedConnections++;
 const target=new URL(req.url);
 if(target.hostname==='claudeff-ci.test'){
  const upstream=http.request({host:'127.0.0.1',port:fixture.address().port,path:target.pathname+target.search,method:req.method,headers:{...req.headers,host:target.host}},r=>{res.writeHead(r.statusCode,r.headers);r.pipe(res)});
  upstream.on('error',()=>{res.writeHead(502);res.end()});req.pipe(upstream);return;
 }
 res.writeHead(503);res.end();
});
proxy.on('connect',(req,socket,head)=>{
 track(socket);
 if(req.headers['proxy-authorization']!==authorization){socket.end('HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic realm="baseline"\r\n\r\n');return}
 authenticatedConnections++;
 const dest=new URL('http://'+req.url);
 // The fault phase disables public probe access, while the local browser fixture remains usable.
 if(rejectProbes&&dest.hostname!=='claudeff-ci.test'){socket.end('HTTP/1.1 503 Service Unavailable\r\n\r\n');return}
 const upstream=track(net.connect(dest.hostname==='claudeff-ci.test'?fixture.address().port:Number(dest.port),dest.hostname==='claudeff-ci.test'?'127.0.0.1':dest.hostname));
 upstream.on('connect',()=>{socket.write('HTTP/1.1 200 Connection Established\r\n\r\n');if(head.length)upstream.write(head);socket.pipe(upstream);upstream.pipe(socket)});
 upstream.on('error',()=>socket.destroy());socket.on('close',()=>upstream.destroy());upstream.on('close',()=>socket.destroy());
});
await new Promise(r=>proxy.listen(0,'127.0.0.1',r));
const [username,password]=auth.split(':');
const settings={proxy_type:'http',host:'127.0.0.1',port:proxy.address().port,username,password};
const report={upstream:'zhom/donutbrowser',tag:'v0.33.0',kind:process.env.DONUT_BASELINE_KIND||'official-release',stage:'start',checks:[],claudeAccountLogin:'not tested',residentialProxy:'not tested',socks5:'not tested',bookmarksAndPasswordFill:'not tested'};
const record=(name,data={})=>{report.checks.push({name,...data});fs.writeFileSync(path.join(output,'result.json'),JSON.stringify(report,null,2));console.log('PASS '+name)};
let app,webview,page,profile,invoke;
const policyPath='HKLM:\\Software\\Policies\\Microsoft\\Edge\\WebView2\\AdditionalBrowserArguments';
const policyName=path.basename(exe);
const policy=spawnSync('powershell.exe',['-NoProfile','-Command',`New-Item -Path '${policyPath}' -Force | Out-Null; New-ItemProperty -Path '${policyPath}' -Name '${policyName}' -Value '--remote-debugging-port=9225 --remote-debugging-address=127.0.0.1' -PropertyType String -Force | Out-Null`],{encoding:'utf8',windowsHide:true});
if(policy.status!==0)throw Error('Cannot configure test WebView debugging policy');
const root=path.join(process.env.RUNNER_TEMP,'donut-baseline-state');fs.mkdirSync(root,{recursive:true});
const alive=pid=>{try{process.kill(pid,0);return true}catch{return false}};
const until=async(fn,ms=60000)=>{const end=Date.now()+ms;while(Date.now()<end){if(await fn())return;await delay(500)}throw Error('Timeout at '+report.stage)};
try{
 const env={...process.env,DONUTBROWSER_DATA_ROOT:root,WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS:'--remote-debugging-port=9225 --remote-debugging-address=127.0.0.1'};
 app=spawn(exe,[],{cwd:path.dirname(exe),env,stdio:'ignore'});
 await until(async()=>{try{webview=await chromium.connectOverCDP('http://127.0.0.1:9225');return true}catch{return false}});
 await until(async()=>{page=webview.contexts()[0].pages().find(p=>!p.url().startsWith('devtools:'));return page&&await page.evaluate(()=>Boolean(window.__TAURI_INTERNALS__?.invoke))});
 invoke=(command,args={})=>page.evaluate(({command,args})=>window.__TAURI_INTERNALS__.invoke(command,args),{command,args});
 record('native upstream Tauri window');
 report.stage='download engine in cloud';
 const versions=await invoke('fetch_browser_versions_cached_first',{browserStr:'wayfern'});assert(versions.length);
 report.engineVersion=versions[0].version;
 try{await invoke('download_browser',{browserStr:'wayfern',version:report.engineVersion})}catch(e){if(!String(e).includes('already being downloaded'))throw e}
 await until(async()=> (await invoke('get_downloaded_browser_versions',{browserStr:'wayfern'})).includes(report.engineVersion),600000);
 await invoke('accept_wayfern_terms');
 record('upstream engine download and normal terms acceptance',{version:report.engineVersion});
 report.stage='authenticated proxy';
 const stored=await invoke('create_stored_proxy',{name:'Cloud authenticated fixture',proxySettings:settings});
 const check=await invoke('check_proxy_validity',{proxyId:stored.id});
 assert(check.is_valid&&net.isIP(check.ip));assert(authenticatedConnections>0);
 record('actual authenticated proxy exit',{latencyMs:check.latency_ms,geoKnown:Boolean(check.country_code)});
 report.stage='create persistent profile';
 profile=await invoke('create_browser_profile_new',{name:'ClaudeFF baseline',browserStr:'wayfern',version:report.engineVersion,releaseType:'stable',proxyId:stored.id,wayfernConfig:{randomize_fingerprint_on_launch:false,geoip:true,restore_session:true},ephemeral:false});
 assert(profile.id);record('native persistent environment creation');
 for(let i=0;i<10;i++){
  report.stage='browser lifecycle '+(i+1);
  const before=visits.length;
  profile=await invoke('launch_browser_profile',{profile,url:i===0?'http://claudeff-ci.test/conversation/baseline':null});
  assert(profile.process_id&&alive(profile.process_id));
  await until(()=>visits.length>before,90000);
  const receipt=visits.at(-1);
  if(i>0){assert.equal(receipt.previous,'persisted');assert(receipt.cookie.includes('baseline=persisted'));assert.equal(receipt.path,'/conversation/baseline')}
  if(i===0){
   record('Chromium page reaches origin through authenticated proxy',{userAgent:receipt.ua});
   report.stage='15 minute native browser stability';
   rejectProbes=true;
   for(let minute=1;minute<=15;minute++){await delay(60000);assert(alive(profile.process_id),'browser exited during probe outage');console.log('Browser alive '+minute+'/15 min')}
   // A new page must still work during the outage: process-alive alone is insufficient.
   const count=visits.length;
   await invoke('open_url_with_profile',{profileId:profile.id,url:'http://claudeff-ci.test/conversation/baseline'});
   await until(()=>visits.length>count);
   rejectProbes=false;record('15 minute survival and usable tab during external probe outage');
  }
  await invoke('kill_browser_profile',{profile});
  await until(()=>!alive(profile.process_id));
  profile=(await invoke('list_browser_profiles')).find(p=>p.id===profile.id);
  record('persistent launch/stop cycle '+(i+1));
 }
 await page.screenshot({path:path.join(output,'original-client.png'),fullPage:true});
 report.stage='complete';report.success=true;
}catch(e){report.success=false;report.error=String(e).split(password).join('[redacted]').slice(0,5000);console.error('FAIL '+report.stage+': '+report.error);process.exitCode=1;
 if(app?.pid){const diagnostic=spawnSync('powershell.exe',['-NoProfile','-File','frontend/tests/desktop-diagnose.ps1','-TaskProcessId',String(app.pid)],{encoding:'utf8',windowsHide:true,timeout:15000});report.desktop=String(diagnostic.stdout||diagnostic.stderr).split(password).join('[redacted]').slice(-10000)}
 if(fs.existsSync(path.join(root,'logs'))){report.logs=fs.readdirSync(path.join(root,'logs')).filter(n=>n.endsWith('.log')).map(n=>fs.readFileSync(path.join(root,'logs',n),'utf8').split(password).join('[redacted]').slice(-10000)).join('\n')}
 if(page)await page.screenshot({path:path.join(output,'failure.png'),fullPage:true}).catch(()=>{});
}finally{
 fs.writeFileSync(path.join(output,'result.json'),JSON.stringify(report,null,2));
 if(profile?.process_id&&invoke)await invoke('kill_browser_profile',{profile}).catch(()=>{});
 if(webview)await webview.close().catch(()=>{});
 if(app?.pid)spawnSync('taskkill',['/PID',String(app.pid),'/T','/F'],{stdio:'ignore'});
 spawnSync('powershell.exe',['-NoProfile','-Command',`Remove-ItemProperty -Path '${policyPath}' -Name '${policyName}' -ErrorAction SilentlyContinue`],{stdio:'ignore',windowsHide:true});
 for(const s of sockets)s.destroy();proxy.closeAllConnections();proxy.close();fixture.closeAllConnections();fixture.close();
}
