import { useDeferredValue, useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { ArrowLeft, ArrowRight, Check, ChevronDown, ChevronRight, Eye, EyeOff, Globe2, Info, LayoutGrid, LogOut, Moon, MoreHorizontal, Plus, RotateCcw, Search, Settings2, Square, Sun, Play, Trash2, X } from 'lucide-react'
import { Button } from './components/ui/button'
import { Modal } from './components/ui/modal'
import { sampleEnvironments, previewNote, type Environment } from './lib/preview'

const logo = `${import.meta.env.BASE_URL}brand/claudeff-icon.png`
type Theme = 'dark' | 'light'
const readTheme = (): Theme => { try { return localStorage.getItem('claudeff.preview.theme') === 'light' ? 'light' : 'dark' } catch { return 'dark' } }

function Brand({ large = false }: { large?: boolean }) {
  return <div className={`brand ${large ? 'brand-large' : ''}`}><img src={logo} alt="" width={large ? 80 : 38} height={large ? 80 : 38} /><span>claude<span className="brand-ff">FF</span></span></div>
}

function Auth({ onEnter }: { onEnter: (name: string) => void }) {
  const [register, setRegister] = useState(false)
  const [name, setName] = useState('体验用户')
  const [password, setPassword] = useState('claudeFF')
  const [showPassword, setShowPassword] = useState(false)
  const submit = (event: FormEvent) => { event.preventDefault(); onEnter(name.trim() || '体验用户'); setPassword('') }
  return <main className="auth-page">
    <div className="auth-card">
      <Brand large />
      <h1>{register ? '准备好你的工作空间' : '欢迎回来'}</h1>
      <p className="auth-subtitle">简单开始，让每个浏览器环境各自独立。</p>
      <div className="segmented auth-tabs" aria-label="选择登录或注册">
        <button aria-pressed={!register} onClick={() => setRegister(false)}>登录</button>
        <button aria-pressed={register} onClick={() => setRegister(true)}>注册</button>
      </div>
      <form onSubmit={submit} className="auth-form">
        <label className="field">用户名<input autoComplete="off" value={name} onChange={(e) => setName(e.target.value)} placeholder="怎么称呼你？" /></label>
        <label className="field">密码<div className="password-field"><input autoComplete="off" type={showPassword ? 'text' : 'password'} value={password} onChange={(e) => setPassword(e.target.value)} placeholder="输入任意内容即可体验" /><button type="button" className="icon-button" aria-label={showPassword ? '隐藏密码' : '显示密码'} onClick={() => setShowPassword(!showPassword)}>{showPassword ? <EyeOff size={18} /> : <Eye size={18} />}</button></div></label>
        {register && <label className="field">确认密码<input type="password" autoComplete="off" placeholder="预览阶段无需验证" /></label>}
        <Button type="submit" className="auth-submit">{register ? '创建并进入工作空间' : '进入工作空间'}<ArrowRight size={16} /></Button>
      </form>
      <div className="auth-hint"><span className="quiet-dot" />开发模式 · 随意输入即可体验，无需真实账号</div>
    </div>
    <p className="auth-footer">独立环境，从容工作。<span>claudeFF 设计预览</span></p>
  </main>
}

function CreateEnvironment({ open, close, onCreate }: { open: boolean; close: () => void; onCreate: (name: string, note: string) => void }) {
  const [step, setStep] = useState(1)
  const [address, setAddress] = useState('')
  const [name, setName] = useState('我的新环境')
  const [note, setNote] = useState('')
  useEffect(() => { if (open) { setStep(1); setAddress(''); setName('我的新环境'); setNote('') } }, [open])
  return <Modal open={open} onClose={close} title="添加一个新环境" description="只需两步，把新的工作空间准备好。" returnFocus="new-environment">
    <div className="steps" aria-label={`第 ${step} 步，共 2 步`}><span className={step === 1 ? 'current' : 'complete'}><b>{step === 2 ? <Check size={12} /> : '1'}</b>连接代理</span><span className="step-line" /><span className={step === 2 ? 'current' : ''}><b>2</b>命名环境</span></div>
    {step === 1 ? <form onSubmit={(e) => { e.preventDefault(); setStep(2) }}>
      <div className="wizard-intro"><div className="soft-icon"><Globe2 size={23} /></div><div><h3>从你的代理开始</h3><p>粘贴代理地址，其他设置按需展开。</p></div></div>
      <label className="field">代理地址<input autoFocus value={address} onChange={(e) => setAddress(e.target.value)} placeholder="例如 proxy.example.com:8080" autoComplete="off" /></label>
      <details className="advanced"><summary>高级选项<ChevronDown size={15} /></summary><div className="advanced-body">
        <label className="field">代理协议<select defaultValue="HTTP"><option>HTTP</option><option>HTTPS</option><option>SOCKS5</option></select></label>
        <div className="form-pair"><label className="field">代理用户名<input placeholder="没有则留空" autoComplete="off" /></label><label className="field">代理密码<input type="password" placeholder="没有则留空" autoComplete="off" /></label></div>
      </div></details>
      <p className="inline-note"><Info size={15} />当前仅预览流程，不连接代理，也不保存代理凭据。</p>
      <div className="modal-actions"><Button variant="ghost" onClick={close}>取消</Button><Button type="submit">预览下一步<ArrowRight size={15} /></Button></div>
    </form> : <form onSubmit={(e) => { e.preventDefault(); onCreate(name.trim(), note.trim()); close() }}>
      <label className="field">环境名称<input autoFocus required pattern=".*\S.*" maxLength={60} value={name} onChange={(e) => setName(e.target.value)} placeholder="给它起一个好记的名字" /></label>
      <label className="field">备注<span className="optional">可选</span><textarea maxLength={240} value={note} onChange={(e) => setNote(e.target.value)} placeholder="这个环境用来做什么？" rows={2} /></label>
      <div className="configuration-note"><Check size={17} /><div><strong>复杂的配置，交给我们</strong><p>正式版本会根据真实检测结果配置语言和时区。</p></div></div>
      <details className="advanced"><summary>查看环境配置<ChevronDown size={15} /></summary><dl className="property-list"><div><dt>代理出口</dt><dd>未检测</dd></div><div><dt>语言 / 时区</dt><dd>等待真实检测</dd></div><div><dt>浏览器数据</dt><dd>每个环境独立保存</dd></div><div><dt>高级指纹</dt><dd>依据内核能力启用</dd></div></dl></details>
      <div className="modal-actions"><Button variant="ghost" onClick={() => setStep(1)}><ArrowLeft size={15} />上一步</Button><Button type="submit">添加预览环境<Plus size={15} /></Button></div>
    </form>}
  </Modal>
}

function SettingsPage({ theme, setTheme, notify, logout }: { theme: Theme; setTheme: (t: Theme) => void; notify: (s: string) => void; logout: () => void }) {
  const [timeout, setTimeout] = useState('15')
  return <div className="settings-content">
    <section className="settings-section"><h2>让工作空间更合心意</h2><p>常用设置，刚刚好。</p>
      <div className="settings-group"><div className="setting-row appearance-row"><div><strong>外观</strong><p>选择你喜欢的明暗，立即生效。</p></div><div className="theme-options">
        <button aria-pressed={theme === 'light'} onClick={() => setTheme('light')}><span className="theme-thumbnail light-thumbnail"><i /><i /><i /></span><span><Sun size={14} />浅色{theme === 'light' && <Check size={14} />}</span></button>
        <button aria-pressed={theme === 'dark'} onClick={() => setTheme('dark')}><span className="theme-thumbnail dark-thumbnail"><i /><i /><i /></span><span><Moon size={14} />深色{theme === 'dark' && <Check size={14} />}</span></button>
      </div></div></div>
    </section>
    <section className="settings-section"><h2>连接与存储</h2><p>默认就很好用，需要时再调整。</p><div className="settings-group">
      <details className="setting-details"><summary><span><Globe2 size={18} />高级设置</span><ChevronDown size={16} /></summary><div className="setting-details-body">
        <div className="setting-row"><div><strong>代理检测超时</strong><p>连接较慢时，可以多等一会。</p></div><select aria-label="代理检测超时" value={timeout} onChange={(e) => { setTimeout(e.target.value); notify('已调整预览设置，未改变真实网络配置。') }}><option value="10">10 秒</option><option value="15">15 秒</option><option value="30">30 秒</option></select></div>
        <div className="setting-row"><div><strong>Chromium 位置</strong><p>接入桌面功能后选择浏览器内核。</p></div><Button variant="secondary" onClick={() => notify('网页预览不访问你的电脑文件；此功能在桌面版接入。')}>选择位置</Button></div>
        <div className="setting-row"><div><strong>本地数据目录</strong><p>浏览器数据将保存在 Windows 应用数据目录。</p></div><Button variant="ghost" onClick={() => notify('网页预览未创建本地浏览器数据。')}>查看位置<ChevronRight size={14} /></Button></div>
      </div></details>
      <div className="setting-row"><div><strong>清理缓存</strong><p>仅清理临时缓存，保留登录状态与环境配置。</p></div><Button variant="secondary" onClick={() => notify('当前没有真实浏览器缓存，这是界面预览。')}>清理</Button></div>
    </div></section>
    <section className="settings-section"><h2>关于</h2><div className="settings-group"><div className="setting-row"><div className="about-brand"><img src={logo} alt="" width="42" height="42" /><div><strong>claudeFF</strong><p>独立浏览器环境管理</p></div></div><span className="version">0.1 · 设计预览</span></div><div className="setting-row"><div><strong>当前体验身份</strong><p>开发模式，不关联真实账号。</p></div><Button variant="ghost" onClick={logout}>退出体验<LogOut size={15} /></Button></div></div></section>
  </div>
}

export default function App() {
  const [theme, setTheme] = useState<Theme>(readTheme)
  const [user, setUser] = useState<string | null>(null)
  const [page, setPage] = useState<'environments' | 'settings'>('environments')
  const [environments, setEnvironments] = useState(sampleEnvironments)
  const [query, setQuery] = useState('')
  const deferredQuery = useDeferredValue(query)
  const [onlyRunning, setOnlyRunning] = useState(false)
  const [creating, setCreating] = useState(false)
  const [editing, setEditing] = useState<Environment | null>(null)
  const [deleting, setDeleting] = useState<Environment | null>(null)
  const [help, setHelp] = useState(false)
  const [toast, setToast] = useState('')
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const heading = useRef<HTMLHeadingElement>(null)
  const notify = (message: string) => { if (toastTimer.current) clearTimeout(toastTimer.current); setToast(message); toastTimer.current = setTimeout(() => setToast(''), 4500) }
  useEffect(() => () => { if (toastTimer.current) clearTimeout(toastTimer.current) }, [])
  useEffect(() => { document.documentElement.dataset.theme = theme; document.documentElement.style.colorScheme = theme; try { localStorage.setItem('claudeff.preview.theme', theme) } catch { /* Theme still works without storage. */ } }, [theme])
  useEffect(() => { if (user) heading.current?.focus() }, [page, user])
  const visible = environments.filter((env) => `${env.name} ${env.note} ${env.region} ${env.ip}`.toLowerCase().includes(deferredQuery.toLowerCase().trim()) && (!onlyRunning || env.status === 'running'))
  const enter = (name: string) => { setUser(name); setPage('environments') }
  const logout = () => { setUser(null); setToast('') }
  const toggleState = (env: Environment) => {
    if (env.status === 'unchecked') { notify('这个预览环境尚未检测，不能启动真实浏览器。'); return }
    setEnvironments((items) => items.map((item) => item.id === env.id ? { ...item, status: item.status === 'running' ? 'stopped' : 'running' } : item))
    notify('已切换演示状态，未启动或关闭真实浏览器。')
  }
  return <>
    <div className="preview-bar"><span><span className="preview-dot" />界面预览<span className="preview-detail"> · 示例数据，未连接浏览器</span></span><div>{user && <button onClick={() => { setEnvironments(sampleEnvironments()); setQuery(''); setOnlyRunning(false); notify('已恢复初始示例。') }}><RotateCcw size={12} />重置示例</button>}<button onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')} aria-label={`切换为${theme === 'dark' ? '浅色' : '深色'}主题`}>{theme === 'dark' ? <Sun size={15} /> : <Moon size={15} />}</button></div></div>
    {!user ? <Auth onEnter={enter} /> : <div className="workspace">
      <a className="skip-link" href="#main-content">跳到主要内容</a>
      <aside className="sidebar"><Brand /><span className="sidebar-caption">工作空间</span><nav aria-label="主导航"><button aria-label="浏览器环境" className={page === 'environments' ? 'active' : ''} onClick={() => setPage('environments')}><LayoutGrid size={18} /><span>浏览器环境</span></button><button aria-label="设置" className={page === 'settings' ? 'active' : ''} onClick={() => setPage('settings')}><Settings2 size={18} /><span>设置</span></button></nav>
        <div className="sidebar-bottom"><button className="preview-help" onClick={() => setHelp(true)}><Info size={15} /><span>关于这次预览</span></button><div className="account"><span className="avatar">{user.slice(0, 1)}</span><div><strong>{user}</strong><span>开发模式</span></div><button className="icon-button" aria-label="退出体验" onClick={logout}><LogOut size={16} /></button></div></div>
      </aside>
      <main className="main-content" id="main-content"><header className="page-header"><div><h1 ref={heading} tabIndex={-1}>{page === 'environments' ? '浏览器环境' : '设置'}</h1><p>{page === 'environments' ? '你的每个工作空间，都井然有序。' : '按你的习惯，轻松调整。'}</p></div>{page === 'environments' && <Button id="new-environment" onClick={() => setCreating(true)}><Plus size={17} /><span>新建环境</span></Button>}</header>
        {page === 'settings' ? <SettingsPage theme={theme} setTheme={setTheme} notify={notify} logout={logout} /> : <div className="environments-content">
          <div className="list-toolbar"><div className="segmented filter-tabs"><button aria-pressed={!onlyRunning} onClick={() => setOnlyRunning(false)}>全部环境<span>{environments.length}</span></button><button aria-pressed={onlyRunning} onClick={() => setOnlyRunning(true)}>运行中</button></div><label className="search-field"><Search size={16} /><span className="sr-only">搜索环境</span><input placeholder="搜索环境" value={query} onChange={(e) => setQuery(e.target.value)} />{query && <button className="icon-button" aria-label="清除搜索" onClick={() => setQuery('')}><X size={14} /></button>}</label></div>
          {visible.length ? <div className="environment-list"><div className="table-heading" aria-hidden="true"><span>环境</span><span>代理 / 地区</span><span>状态</span><span>操作</span></div><ul>{visible.map((env) => <li key={env.id} className="environment-row">
            <div className="environment-identity"><span className={`environment-icon ${env.tone}`}><Globe2 size={22} /></span><div><h2>{env.name}</h2><p title={env.note}>{env.note || '还没有添加备注'}</p></div></div>
            <div className="environment-network"><span className="ip">{env.ip}</span><span><span className="country-code">{env.code}</span>{env.region}</span></div>
            <span className={`status status-${env.status}`}><i />{env.status === 'running' ? '运行中' : env.status === 'unchecked' ? '待检测' : '已关闭'}</span>
            <div className="row-actions"><Button variant="secondary" className={env.status === 'running' ? 'stop-button' : ''} onClick={() => toggleState(env)} aria-label={`${env.status === 'running' ? '关闭' : '启动'}${env.name}`}>{env.status === 'running' ? <Square size={13} /> : <Play size={13} />}{env.status === 'running' ? '关闭' : '启动'}</Button><button id={`edit-${env.id}`} className="icon-button" aria-label={`编辑${env.name}`} onClick={() => setEditing({ ...env })}><MoreHorizontal size={19} /></button></div>
          </li>)}</ul><div className="list-footer"><span>{visible.length} 个环境<span className="footer-separator">·</span>以上为界面示例</span><button onClick={() => { setEnvironments([]); setQuery(''); setOnlyRunning(false) }}>查看空状态<ChevronRight size={13} /></button></div></div> : <div className="empty-state"><span className="empty-icon"><LayoutGrid size={30} /></span><h2>{environments.length ? '没有找到这个环境' : '给下一份工作，留一个独立空间'}</h2><p>{environments.length ? '换一个名称、地区或 IP 试试。' : '添加代理，为它起个名字，就可以准备开始了。'}</p><Button variant={environments.length ? 'secondary' : 'primary'} onClick={() => { if (environments.length) { setQuery(''); setOnlyRunning(false) } else setCreating(true) }}>{environments.length ? '显示全部环境' : '创建第一个环境'}{!environments.length && <Plus size={16} />}</Button></div>}
          <div className="workspace-note"><span className="note-line" /><p>独立的浏览数据，熟悉的工作方式。</p><span className="note-line" /></div>
        </div>}
      </main>
    </div>}
    <CreateEnvironment open={creating} close={() => setCreating(false)} onCreate={(name, note) => { setEnvironments((items) => [...items, { id: `preview-${crypto.randomUUID()}`, name, note, region: '等待检测', code: '—', ip: '未检测', status: 'unchecked', language: '待配置', timezone: '待配置', tone: 'blue' }]); setQuery(''); setOnlyRunning(false); notify('已添加预览环境。代理与浏览器功能将在后续接入。') }} />
    <Modal open={!!editing} onClose={() => setEditing(null)} title="环境详情" description="一个好记的名字，让工作更有条理。" returnFocus={editing ? `edit-${editing.id}` : 'new-environment'}>{editing && <form onSubmit={(e) => { e.preventDefault(); setEnvironments((items) => items.map((item) => item.id === editing.id ? { ...editing, name: editing.name.trim() } : item)); setEditing(null); notify('已更新预览中的名称与备注。') }}>
      <label className="field">环境名称<input required maxLength={60} value={editing.name} onChange={(e) => setEditing({ ...editing, name: e.target.value })} /></label><label className="field">备注<textarea rows={2} maxLength={240} value={editing.note} onChange={(e) => setEditing({ ...editing, note: e.target.value })} /></label>
      <details className="advanced"><summary>更多环境信息<ChevronDown size={15} /></summary><dl className="property-list"><div><dt>示例出口 IP</dt><dd>{editing.ip}</dd></div><div><dt>语言</dt><dd>{editing.language}</dd></div><div><dt>时区</dt><dd>{editing.timezone}</dd></div><div><dt>住宅属性</dt><dd>未验证</dd></div></dl></details>
      <div className="modal-actions"><Button variant="ghost" className="destructive-text" onClick={() => { setDeleting(editing); setEditing(null) }}><Trash2 size={15} />删除环境</Button><Button type="submit">保存修改</Button></div>
    </form>}</Modal>
    <Modal open={!!deleting} onClose={() => setDeleting(null)} title="删除这个预览环境？" description={`“${deleting?.name ?? ''}”将从示例列表移除。这里没有真实浏览器数据。`} returnFocus="new-environment"><div className="modal-actions"><Button variant="secondary" onClick={() => setDeleting(null)}>保留环境</Button><Button variant="danger" onClick={() => { setEnvironments((items) => items.filter((item) => item.id !== deleting?.id)); setDeleting(null); notify('预览环境已删除。') }}>删除预览环境</Button></div></Modal>
    <Modal open={help} onClose={() => setHelp(false)} title="先体验，再打磨" description="这是一份可以操作的界面设计预览。"><div className="help-copy"><p>{previewNote}</p><p>你可以体验登录、搜索、主题切换、创建、编辑与删除。输入的密码和代理凭据不会被保存或上传。</p><p>设计风格确认后，再开始真实浏览器、代理和本地存储的开发。卡密及用户管理面板放在最后。</p></div><div className="modal-actions"><Button onClick={() => setHelp(false)}>知道了</Button></div></Modal>
    {toast && <div className="toast" role="status"><Info size={17} /><span>{toast}</span><button className="icon-button" aria-label="关闭提示" onClick={() => setToast('')}><X size={15} /></button></div>}
  </>
}
