import { useEffect, useRef } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { isNative, terminal } from '../lib/api'
export function TerminalPane({ running }: { running: boolean }) {
 const element = useRef<HTMLDivElement>(null); const instance = useRef<Terminal | null>(null); const active = useRef(running)
 useEffect(() => { active.current = running; if (instance.current) { instance.current.options.cursorBlink = running; if (!isNative() && running) instance.current.write('\r\n预览 > ') } }, [running])
 useEffect(() => {
  if (!element.current) return
  const term = new Terminal({ fontFamily: 'Cascadia Code, Consolas, monospace', fontSize: 14, cursorBlink: active.current, theme: { background: '#141517', foreground: '#e9e7e2', cursor: '#d97757' }, scrollback: 3000 }); instance.current = term
  const fit = new FitAddon(); term.loadAddon(fit); term.open(element.current); fit.fit()
  term.writeln(isNative() ? '\x1b[90m终端已准备。启动后显示真实 Claude Code 输出。\x1b[0m' : '\x1b[90m界面预览 · 不是实际 Claude Code 会话\x1b[0m')
  if (!isNative()) { term.writeln(''); term.writeln('  Claude Code'); term.writeln('  WSL2 · 共享全局连接 · 独立项目空间'); term.writeln(''); term.write('\x1b[90m准备环境后，点击「启动终端」。\x1b[0m') }
  term.attachCustomKeyEventHandler((event) => { if(event.type !== 'keydown') return true
   if(event.ctrlKey && event.shiftKey && event.code === 'KeyC') { void navigator.clipboard?.writeText(term.getSelection()); return false }
   if(event.ctrlKey && event.shiftKey && event.code === 'KeyV') { void navigator.clipboard?.readText().then((text) => { if (isNative() && active.current) void terminal('terminal.write', { data: text }) }); return false }
   return true
  })
  const data = term.onData((value) => { if (!active.current) return; if (isNative()) void terminal('terminal.write', { data: value }); else { if (value === '\r') { term.write('\r\n（预览不会执行命令）\r\n预览 > ') } else if (value === '\u007f') term.write('\b \b'); else if (value >= ' ') term.write(value) } })
  const off = window.runtime?.EventsOn('terminal:data', (value) => term.write(value))
  const resize = new ResizeObserver(() => { fit.fit(); if (isNative()) void terminal('terminal.resize', { cols: term.cols, rows: term.rows }) }); resize.observe(element.current)
  return () => { off?.(); data.dispose(); resize.disconnect(); instance.current = null; term.dispose() }
 }, [])
 return <div className="terminal-pane" ref={element} aria-label="Claude Code 终端" />
}
