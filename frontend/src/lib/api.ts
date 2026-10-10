export type Phase = 'idle' | 'busy' | 'ready' | 'running' | 'failed'
export type Observation = { ip: string; country: string; countryCode?: string; city?: string; region?: string; isp?: string; endpoint?: string; timezone: string; latency: number; at: string; revision: number }
export type Status = { version?: string; phase: Phase; message: string; observation?: Observation }
export type ProxyConfig = { protocol: string; host: string; port: number; username: string; password?: string }
export type Profile = { id: string; revision: number; source: string; seed: string; language: string; timezone: string; createdAt: string }
export type Snapshot = { browserSupported?: boolean; platform?: string; project?: string; proxy: ProxyConfig; hasPassword: boolean; revision: number; profile?: Profile; browser: Status; code: Status; network: Status; kernel: boolean; native: boolean; dataDir: string; previous?: Observation; authRequired?: boolean; authorized?: boolean; fingerprint?: Record<string, unknown> }
type Native = { Invoke: (action: string, payload: string) => Promise<string> }
declare global { interface Window { go?: { main?: { App?: Native } }; runtime?: { EventsEmit?: (name: string, ...data: unknown[]) => void; EventsOn: (name: string, cb: (data: string) => void) => () => void } } }
export const isNative = () => Boolean(window.go?.main?.App)
const idle = (): Status => ({ phase: 'idle', message: '尚未启动' })
let demo: Snapshot = { proxy: { protocol: 'http', host: '', port: 8080, username: '' }, hasPassword: false, revision: 0, browser: idle(), code: idle(), network: { phase: 'idle', message: '先添加你的代理' }, kernel: true, native: false, dataDir: '预览不访问电脑文件' }
const wait = () => new Promise((r) => setTimeout(r, 350))
const observation = (): Observation => ({ ip: '203.0.113.12', country: '示例地区', timezone: 'UTC', latency: 126, at: new Date().toISOString(), revision: demo.revision })
function ensure(source: string) { demo.profile ??= { id: 'preview-001', revision: 1, source, seed: '102468', language: 'en-US', timezone: 'UTC', createdAt: new Date().toISOString() } }
export async function invoke(action: string, payload: unknown = {}): Promise<Snapshot> {
 const native = window.go?.main?.App
 if (native) return JSON.parse(await native.Invoke(action, JSON.stringify(payload))) as Snapshot
 if (action === 'snapshot') return structuredClone(demo)
 await wait()
 if (action === 'proxy.save' || action === 'proxy.saveAndCheck') { const p = payload as ProxyConfig; if (!p.host || !p.port) throw new Error('请填写代理地址和端口'); demo.proxy = { ...p, password: undefined }; demo.hasPassword = !!p.password; demo.revision++; demo.network = { phase: 'idle', message: '已修改，请重新检测' }; demo.browser = idle(); demo.code = idle() }
 if (action === 'proxy.check' || action === 'proxy.saveAndCheck') demo.network = { phase: 'ready', message: '示例连接结果', observation: observation() }
 if (action === 'browser.start' || action === 'profile.create') { if (demo.network.phase !== 'ready') throw new Error('请先在全局连接中检测代理'); ensure('browser'); demo.browser = { phase: 'running', message: '示例浏览器状态', observation: observation() } }
 if (action === 'browser.stop') demo.browser = idle()
 if (action === 'code.install') { if (demo.network.phase !== 'ready') throw new Error('请先配置并检测全局代理'); demo.code = { phase: 'ready', message: '演示安装完成', observation: observation() } }
 if (action === 'code.start') { if (demo.code.phase !== 'ready') throw new Error('请先准备 Code'); demo.code = { ...demo.code, phase: 'running', message: '终端交互演示' } }
 if (action === 'code.stop') demo.code = { ...demo.code, phase: 'ready', message: '演示已停止' }
 if (action === 'profile.save') { if (demo.profile) { Object.assign(demo.profile, payload); demo.profile.revision++; demo.browser = idle() } }
 if (action === 'demo.fail') demo.code = { phase: 'failed', message: '示例：下载失败，请重试。' }
 if (action === 'project.choose') demo.project = '示例项目目录'
 if (action === 'demo.reset') { demo = { ...demo, profile: undefined, proxy: { protocol: 'http', host: '', port: 8080, username: '' }, hasPassword: false, revision: 0, network: { phase: 'idle', message: '先添加你的代理' }, browser: idle(), code: idle() } }
 return structuredClone(demo)
}
export async function terminal(action: string, payload: unknown = {}) {
 const native = window.go?.main?.App; if (native) await native.Invoke(action, JSON.stringify(payload))
}

export const providerLink = 'https://www.ipdeep.cn/?extendid=30d6N2VjYjI3MzQ4NDUyNTY3OTQxMTg5'
export async function openProvider() {const n=window.go?.main?.App;if(n) await n.Invoke('provider.open','{}')}
export async function parseProxy(value: string | undefined, protocol: string): Promise<ProxyConfig> {
 const n=window.go?.main?.App
 if(n) return JSON.parse(await n.Invoke(value===undefined?'proxy.clipboard':'proxy.parse',JSON.stringify({value,protocol}))) as ProxyConfig
 const raw=(value===undefined?await navigator.clipboard.readText():value).trim()
 let p:ProxyConfig
 try {
  if(raw.includes('://')||raw.includes('@')) {const u=new URL(raw.includes('://')?raw:`${protocol}://${raw}`);if(u.pathname&&u.pathname!=='/'||u.search||u.hash)throw new Error();p={protocol:u.protocol.slice(0,-1).replace('socks5h','socks5'),host:u.hostname.replace(/^\[|\]$/g,''),port:Number(u.port),username:decodeURIComponent(u.username),password:decodeURIComponent(u.password)}}
  else {const m=raw.match(/^(\[[^\]]+\]|[^:]+):(\d+)(?::([^:]*):(.*))?$/);if(!m)throw new Error();p={protocol,host:m[1].replace(/^\[|\]$/g,''),port:Number(m[2]),username:m[3]||'',password:m[4]||''}}
  if(!['http','https','socks5'].includes(p.protocol)||!p.host||p.port<1||p.port>65535||/[\r\n\t]/.test(raw))throw new Error()
  return p
 }catch{throw new Error('代理格式无法识别，请粘贴 地址:端口:用户名:密码 或完整代理 URL')}
}
