"""Cloud-only: download complete official runtime assets, verify pinned digests."""
import hashlib, json, os, pathlib, urllib.request, zipfile, subprocess
if os.environ.get('GITHUB_ACTIONS') != 'true':
    raise SystemExit('Runtime packaging only runs in GitHub Actions')
root=pathlib.Path('build/bin');cache=pathlib.Path(os.environ['RUNNER_TEMP'])/'claudeff-runtimes';cache.mkdir(exist_ok=True)
assets=[('fingerprint-chromium','148.0.7778.215','https://github.com/adryfish/fingerprint-chromium/releases/download/148.0.7778.215/ungoogled-chromium_148.0.7778.215-1.1_windows_x64.zip','9ef3f471b7a6641b4224532522b29141ce3746e27d55788d88e2fd951f362579','kernel.zip'),('WebView2','155.0.4283.45','https://msedge.sf.dl.delivery.mp.microsoft.com/filestreamingservice/files/01b0fbd9-6ea3-4e12-818a-a71f73f3855b/Microsoft.WebView2.FixedVersionRuntime.155.0.4283.45.x64.cab','ec533bf1b7354d288703cf821a2d640d0130108224f93e330bb9c09faf3f8010','webview2.cab')]
manifest=[]
for name,version,url,expected,filename in assets:
    path=cache/filename
    if not path.exists():
        with urllib.request.urlopen(url,timeout=120) as r,path.open('wb') as f:
            while b:=r.read(1<<20):f.write(b)
    h=hashlib.file_digest(path.open('rb'),'sha256').hexdigest()
    if expected and h!=expected:raise SystemExit(name+' SHA-256 mismatch')
    manifest.append(dict(name=name,version=version,url=url,sha256=h))
    if filename.endswith('.zip'):
        with zipfile.ZipFile(path) as z:z.extractall(root/'browser')
    else:
        dest=cache/'webview2-extracted';dest.mkdir(exist_ok=True)
        subprocess.run(['expand.exe',str(path),'-F:*',str(dest)],check=True,stdout=subprocess.DEVNULL)
        exe=next(dest.rglob('msedgewebview2.exe'))
        import shutil
        if (root/'webview2').exists():shutil.rmtree(root/'webview2')
        shutil.copytree(exe.parent,root/'webview2')
(root/'RUNTIMES.json').write_text(json.dumps(manifest,indent=2),encoding='utf-8')
print('Complete Chromium and fixed WebView2 runtimes packaged.')
