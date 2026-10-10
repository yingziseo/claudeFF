package browser

import "runtime"

// The pinned upstream macOS 148 DMG contains an arm64-only native kernel.
// Intel macOS retains native Claude Code, without substituting plain Chromium.
func Supported() bool { return runtime.GOOS != "darwin" || runtime.GOARCH == "arm64" }
