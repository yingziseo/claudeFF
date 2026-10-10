"""Publish only complete, matching-source cloud results; never overwrite an old tag."""
import hashlib,json,os,pathlib,re,shutil,subprocess,zipfile
if os.environ.get('GITHUB_ACTIONS')!='true':raise SystemExit('Cloud-only publishing')
repo=os.environ['GITHUB_REPOSITORY'];source=None
expected={'WINDOWS_RUN':'Windows portable client','MACOS_RUN':'macOS native client','CORE_RUN':'Core validation','FRONTEND_RUN':'Frontend preview','LICENSE_RUN':'License service and panel'}
for key,name in expected.items():
 run=os.environ[key]
 if not re.fullmatch(r'\d+',run):raise SystemExit('Invalid workflow identifier')
 data=json.loads(subprocess.check_output(['gh','api',f'repos/{repo}/actions/runs/{run}']))
 if data['name']!=name or data['conclusion']!='success':raise SystemExit('Required validation did not succeed: '+name)
 if source is not None and source!=data['head_sha']:raise SystemExit('Required checks do not refer to the same source')
 source=data['head_sha']
tag=os.environ['RELEASE_TAG']
if not re.fullmatch(r'v\d+\.\d+\.\d+-dev\.\d+',tag):raise SystemExit('Invalid development tag')
if subprocess.run(['gh','release','view',tag],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode==0:raise SystemExit('Release already exists; do not overwrite')
root=pathlib.Path('release-assets');root.mkdir()
for run,name in [(os.environ['WINDOWS_RUN'],'windows-client'),(os.environ['MACOS_RUN'],'macos-intel'),(os.environ['MACOS_RUN'],'macos-apple-silicon')]:
 dest=pathlib.Path('artifacts')/name
 subprocess.run(['gh','run','download',run,'-n',name,'-D',str(dest)],check=True)
 for pattern in ['*.zip','*.sha256']:
  for p in dest.glob(pattern):shutil.copy2(p,root/p.name)
 if name=='windows-client':
  report=next(dest.rglob('result.json'));shutil.copy2(report,root/'windows-native-verification.json')
  runtimes=next(dest.rglob('RUNTIMES.json'));shutil.copy2(runtimes,root/'RUNTIMES.json')
checks=[]
for p in sorted(root.glob('*.zip')):
 with p.open('rb') as f:digest=hashlib.file_digest(f,'sha256').hexdigest()
 claimed=p.with_suffix('.sha256').read_text().split()[0]
 if digest!=claimed:raise SystemExit('Archive checksum mismatch')
 checks.append(f'{digest}  {p.name}')
 if 'windows' in p.name:
  with zipfile.ZipFile(p) as z:
   names=z.namelist()
   if 'claudeFF.exe' not in names or 'webview2/msedgewebview2.exe' not in names or not any(n.startswith('browser/') and n.endswith('/chrome.exe') for n in names):raise SystemExit('Windows package misses an embedded runtime')
   if any(n.startswith('sandbox/') or 'browser-profile/' in n for n in names):raise SystemExit('Obsolete or user data included')
   for row in z.read('SHA256SUMS.txt').decode('utf-8-sig').splitlines():
    h,name=row.split('  ',1)
    with z.open(name) as f:actual=hashlib.file_digest(f,'sha256').hexdigest()
    if h!=actual:raise SystemExit('Individual package file checksum mismatch')
print('\n'.join(checks))
(root/'SHA256SUMS.txt').write_text('\n'.join(checks)+'\n')
notes=f'''完整资源开发体验版：浏览器与 Windows WebView2 直接随包，仅 Claude Code 在线安装。

- 一键粘贴代理、真实地区/请求耗时、明确错误及备用检测站。
- 指纹浏览器保留原生 UI、独立 profile，并恢复上次标签/对话。
- Claude Code 官方原生安装、共用代理、本机项目和真实终端；取消 WSL/虚拟机/沙箱与指纹同步。
- 精简界面，保留必要操作及 IPDeep 推荐入口。

Windows x64 完整版、Mac Apple Silicon 完整版；Intel Mac 包当前提供原生 Code，内置指纹浏览器不支持 Intel。

云端原生程序、用户授权代理和内核链路证据见附件与 docs 验证报告。未用真实 Claude 账号执行登录或模型请求，不宣称所有网络/权限/恢复场景都验收完成。macOS 开发包未完成开发者签名/公证；生产卡密服务未部署，产品登录仍为开发交互。

构建源提交：{source}
'''
body=pathlib.Path('release-notes.md');body.write_text(notes)
subprocess.run(['gh','release','create',tag,*[str(p) for p in sorted(root.iterdir())],'--target',source,'--prerelease','--title','claudeFF 0.3 · 完整内核与原生 Code','--notes-file',str(body)],check=True)
