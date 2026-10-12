#!/usr/bin/env bash
set -euo pipefail
if [[ "${GITHUB_ACTIONS:-}" != "true" ]]; then
  echo "AGENTS.md requires building and packaging in GitHub Actions."
  exit 1
fi
cd "$(dirname "$0")/.."
DOTNET_BIN="${DOTNET_BIN:-dotnet}"
"$DOTNET_BIN" run --project tests/ClaudeFF.Tests -c Release
out="$PWD/artifacts/claudeFF-win-x64"
mkdir -p "$out"
for project in Probe Relay App; do
  "$DOTNET_BIN" publish "src/ClaudeFF.$project" -c Release -r win-x64 --self-contained true -p:DebugType=None -p:DebugSymbols=false -o "$out" --nologo
done
cp docs/Windows使用说明.md "$out/使用说明.md"
cp docs/第三方与接口来源.md "$out/第三方与接口来源.md"
cp docs/实现与验证边界.md "$out/实现与验证边界.md"
cp docs/界面预览.html "$out/界面预览.html"
cp docs/assets/LUCIDE-LICENSE.txt "$out/LUCIDE-LICENSE.txt"
cp scripts/windows/* "$out/"
python - <<'PY'
from pathlib import Path
import hashlib,zipfile
root=Path('artifacts/claudeFF-win-x64')
for name in ['ClaudePrivacy.exe','PrivacyAudit.exe','LocalProxy.exe']:
    assert (root/name).read_bytes()[:2]==b'MZ',name+' is not a Windows executable'
manifest='\n'.join(hashlib.sha256(p.read_bytes()).hexdigest()+'  '+p.relative_to(root).as_posix() for p in sorted(root.rglob('*')) if p.is_file() and p.name!='SHA256SUMS.txt')+'\n'
(root/'SHA256SUMS.txt').write_text(manifest)
archive=Path('artifacts/claudeFF-win-x64.zip')
with zipfile.ZipFile(archive,'w',zipfile.ZIP_DEFLATED,compresslevel=6) as z:
    for p in sorted(root.rglob('*')):
        if p.is_file():z.write(p,Path('claudeFF')/p.relative_to(root))
archive.with_suffix('.zip.sha256').write_text(hashlib.sha256(archive.read_bytes()).hexdigest()+'  '+archive.name+'\n')
print(archive.resolve(), archive.stat().st_size)
PY
