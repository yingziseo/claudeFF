# claudeFF

**2026-10-12：正在按新的实机反馈重新评估完整指纹浏览器基座。** 用户报告 V3 代理检测和浏览器秒退出，本轮尚未修复或发布新包。源码评估首选 Donut、备选 CloakBrowser Manager；稳定性待原版 Windows 对比验证，见 [基座评估](docs/指纹浏览器基座选型评估.md)。以下 V3 下载/测试供追溯。

单账号 Claude 工作空间：完整指纹浏览器 + 官方原生 Claude Code，共用一份全局代理。主要交付 Windows 11 x64，增加 macOS 支持。Claude 桌面版预留“正在开发中”。

**[v0.3.0-dev.1 已发布](https://github.com/yingziseo/claudeFF/releases/tag/v0.3.0-dev.1)**，真实 Windows 客户端代理/内核及 Mac 原生 Code 云测通过：内核和 Windows WebView2 直接随 ZIP，浏览器恢复上次标签与对话；代理支持一键粘贴、真实地区/耗时与具体失败原因。Code 正常原生安装，不再需要 WSL2、虚拟机或沙箱。

- [Windows 完整 ZIP](https://github.com/yingziseo/claudeFF/releases/download/v0.3.0-dev.1/claudeFF-windows-x64.zip)（约 491 MiB）
- [Mac ARM / Intel 下载](https://github.com/yingziseo/claudeFF/releases/tag/v0.3.0-dev.1)（Intel 当前仅 Code）
- [网页设计预览](https://yingziseo.github.io/claudeFF/)（示例数据）
- [开发计划](docs/claudeFF开发项目计划.md) / [真实进度与下载](docs/开发进度与验证报告.md)
- [Windows 说明](docs/Windows运行说明.md) / [macOS 说明](docs/macOS运行说明.md)
- [协作规则](AGENTS.md) / [工作日志](工作日志.md) / [依赖来源](docs/第三方许可与来源.md)

所有依赖安装、构建和测试使用 GitHub Actions，禁止在当前服务器构建。Go/Wails 2 + React/TypeScript + SQLite；fingerprint-chromium 完整成品；Windows DPAPI/macOS Keychain 保护代理秘密。只修改本应用启动进程的代理环境，不改系统代理。

开发包产品登录可随意输入；官方 Claude 授权自行完成。永久联网卡密及轻量“卡密和用户管理面板”代码已存在，生产尚未部署，正式商业规则最后收敛。此项目是第三方工具，非官方客户端。旧 v0.2.0-dev.1 仅供历史追溯，不代表本次修复交付。
