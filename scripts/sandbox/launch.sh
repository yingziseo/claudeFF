#!/bin/bash
set -euo pipefail
[ "$(id -un)" = claudeff ] || { echo 'must run as unprivileged claudeff'; exit 1; }
# /mnt, Windows interop sockets, other user homes and arbitrary /run are absent.
# User namespaces and capability dropping prevent setuid binaries from gaining host root.
work=/workspace
if [ -r /opt/claudeff/project-path ]; then
 candidate=$(cat /opt/claudeff/project-path)
 [[ "$candidate" =~ ^/workspace/project-[0-9]+$ ]] && [ -d "$candidate" ] || exit 1
 work="$candidate"
fi
opts=(--die-with-parent --new-session --unshare-user --unshare-pid --unshare-ipc --unshare-uts --unshare-cgroup-try --disable-userns --cap-drop ALL --hostname claudeff
 --ro-bind /usr /usr --ro-bind /bin /bin --ro-bind /lib /lib --ro-bind /lib64 /lib64
 --ro-bind /etc/ssl /etc/ssl --ro-bind /etc/passwd /etc/passwd --ro-bind /etc/group /etc/group
 --ro-bind /etc/localtime /etc/localtime --ro-bind-try /etc/locale.alias /etc/locale.alias
 --proc /proc --dev /dev --tmpfs /tmp
 --bind /home/claudeff /home/claudeff --bind /workspace /workspace
 --chdir "$work" --clearenv)
# Pass only the explicitly managed runtime values. No inherited Windows environment.
opts+=(--setenv HOME /home/claudeff --setenv USER claudeff --setenv PATH /home/claudeff/.local/bin:/usr/local/bin:/usr/bin:/bin
 --setenv LANG "${CLAUDEFF_LANG:?}" --setenv LC_ALL "${CLAUDEFF_LANG:?}" --setenv TZ "${CLAUDEFF_TZ:?}"
 --setenv HTTPS_PROXY "${CLAUDEFF_PROXY:?}" --setenv HTTP_PROXY "${CLAUDEFF_PROXY:?}" --setenv NO_PROXY ''
 --setenv CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC 1)
exec bwrap "${opts[@]}" -- "$@"
