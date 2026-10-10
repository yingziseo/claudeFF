import hashlib,pathlib,sys,zipfile
root=pathlib.Path(sys.argv[1]);output=pathlib.Path(sys.argv[2]);rows=[]
for p in sorted(root.rglob('*')):
 if p.is_file() and p.name!='SHA256SUMS.txt':
  with p.open('rb') as f:h=hashlib.file_digest(f,'sha256').hexdigest()
  rows.append(f'{h}  {p.relative_to(root).as_posix()}')
(root/'SHA256SUMS.txt').write_text('\n'.join(rows)+'\n',encoding='utf-8')
with zipfile.ZipFile(output,'w',zipfile.ZIP_DEFLATED,compresslevel=5,allowZip64=True) as z:
 for p in sorted(root.rglob('*')):
  if p.is_file():z.write(p,p.relative_to(root))
with output.open('rb') as f:h=hashlib.file_digest(f,'sha256').hexdigest()
output.with_suffix('.sha256').write_text(f'{h}  {output.name}\n',encoding='utf-8')
print(f'Packaged {output.name}: {output.stat().st_size} bytes, SHA256 {h}')
