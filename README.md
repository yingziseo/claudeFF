# claudeFF

一个 Windows 桌面隐私工具：应用访问限制后，在同一窗口点击“验证防护”，用实际尝试的结果核对效果。保留真实硬件设备信息，不伪造设备、系统语言或地理位置。

当前为 **0.1.2 原型**。Linux 上可编译并测试公共逻辑；Windows 驱动、客户端兼容性和真实界面仍需在目标机运行验证。没有账号防封保证，也没有证据表明所限制的数据必然导致封号。

## 使用

完整解压 `artifacts/claudeFF-win-x64.zip`，运行 `ClaudePrivacy.exe`。不需要另装 .NET；需要单独安装官方 x64 Sandboxie。`PrivacyAudit.exe` 与 `LocalProxy.exe` 是后台组件，不能删，也不需要手动打开。

1. 选择 Sandboxie 安装目录，选择要限制的浏览器资料/私人目录。
2. 保留已有全局代理，或在网络页配置实验性的沙箱 TCP 代理。
3. 点击“开启防护”，再点击“验证防护”。**配置已应用不等于验证通过。**
4. 安装 Claude 后选择实际 `.exe`，通过“启动 Claude”启动，再自行测试登录、聊天和附件。

详细步骤：[Windows 使用说明](docs/Windows使用说明.md)。[网页界面预览](docs/界面预览.html)只展示布局，不执行真实防护。

## 实现范围

| 功能 | 本版行为 |
|---|---|
| 浏览器/私人目录限制 | 对已发现或指定的目录应用 Sandboxie 访问拒绝规则；不读取浏览器秘密 |
| 一体化验证 | 独立后台探针；宿主基线与沙箱结果对照；运行时核对探针所属沙箱 |
| 注册表规则验证 | 验证自建测试键与已应用规则的浏览器偏好键，不宣称保护所有敏感注册表 |
| 沙箱 TCP 代理 | 本地桥接至住宅 SOCKS5；DPAPI 保存密码；运行时随机挑战验证接管及出口连通 |
| 现有代理/IP 查询 | 不修改第三方代理；仅显示测试请求的实际出口 |
| 可选数据使用 | 对可识别的官方模型改进开关使用 UI Automation 关闭、读回和恢复；需打开 Privacy 页面 |
| 记录与恢复 | 脱敏的验证报告、写入前恢复记录、冲突检测；不删除沙箱数据 |

**未提供的能力不进入验证清单。** 本版没有系统语言读取全面阻止、全部遥测关闭、全局透明代理、UDP/DNS 接管、内核级防泄漏或全天候 Claude 资源访问审计。账户模型改进偏好不等于运行遥测开关。现有测试报告记录探针访问，不冒充 Claude 的完整行为日志。

代理使用 Sandboxie 的受支持 Winsock TCP 重定向和本地 SOCKS 桥接。未选择有效代理规则、低层旁路、UDP/DNS、localhost 与不同地址族都存在独立边界。因此不能把 TCP 测试通过写成“真实 IP 永不泄漏”。一般使用者优先沿用已有全局代理。

## 构建与测试

要求 .NET 10 SDK；Windows 目标可在 Linux 上交叉编译。

```bash
dotnet build ClaudeFF.sln -c Release
dotnet run --project tests/ClaudeFF.Tests -c Release
./scripts/package.sh
```

Windows 打包：`./scripts/package.ps1`。CI 提供 Windows 编译及公共逻辑测试，不会自动登录 Claude，也不能代替真实驱动测试。

目录：

- `ClaudeFF.App`：唯一面向用户的 WPF 窗口。
- `ClaudeFF.Windows`：Sandboxie、凭证、策略恢复、隐私开关和验证编排。
- `ClaudeFF.Probe`：只读访问探针；仅读取自建测试文件内容。
- `ClaudeFF.Relay`：可选的本地 TCP 代理后台组件。
- `ClaudeFF.Core`：模型、验证判定、SOCKS 协议与出口查询。
- `tests`：真假通过判定、输入校验、恢复冲突、SOCKS 握手与失败路径测试。

[技术需求](docs/claudeFFv1.0.md)描述目标范围；本 README 与[实现与验证边界](docs/实现与验证边界.md)描述实际交付能力，两者不能混为已完成事项。
