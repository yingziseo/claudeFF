"""Collect installed upstream license/notice texts for cloud-built development packages."""
import json
import os
import subprocess
import sys
from pathlib import Path


if os.environ.get("GITHUB_ACTIONS") != "true":
    raise SystemExit("Run only in GitHub Actions after cloud dependency installation.")

sections = ["claudeFF dependency notices\n\nCollected from the exact installed module/npm versions. "
            "This inventory is not an authorization to use the Claude brand. "
            "Chromium, Ubuntu and Claude Code are downloaded separately at runtime. "
            "See THIRD-PARTY.md / the source repository for those sources."]
inventory = []
missing = []


def collect(label, directory):
    files = []
    if directory and directory.is_dir():
        for entry in sorted(directory.iterdir()):
            if entry.is_file() and entry.name.upper().startswith(("LICENSE", "LICENCE", "NOTICE", "COPYING", "COPYRIGHT")):
                files.append(entry)
    inventory.append(label)
    if not files:
        missing.append(label)
    for entry in files:
        sections.append(f"{label} / {entry.name}\n{'=' * 72}\n{entry.read_text(encoding='utf-8', errors='replace')}")


collect("Go standard library", Path(subprocess.check_output(["go", "env", "GOROOT"], text=True).strip()))
raw = subprocess.check_output(["go", "list", "-m", "-json", "all"], text=True)
decoder = json.JSONDecoder()
while raw.strip():
    module, end = decoder.raw_decode(raw.lstrip())
    raw = raw.lstrip()[end:]
    if not module.get("Main"):
        collect(module["Path"] + "@" + module.get("Version", ""), Path(module["Dir"]) if module.get("Dir") else None)

lock = json.loads(Path("frontend/package-lock.json").read_text(encoding="utf-8"))
for path, package in sorted(lock["packages"].items()):
    if path:
        collect("npm " + path.rsplit("node_modules/", 1)[-1] + "@" + package.get("version", ""), Path("frontend") / path)

sections.insert(1, "Dependency inventory (including build/test dependencies):\n" + "\n".join(inventory))
if missing:
    sections.insert(2, "No root-level notice found for the following entries; review their upstream/package metadata before commercial release:\n" + "\n".join(missing))
output = Path(sys.argv[1])
output.parent.mkdir(parents=True, exist_ok=True)
output.write_text("\n\n".join(sections), encoding="utf-8")
print(f"Collected notices for {len(inventory)} installed dependency entries.")
