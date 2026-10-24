# 开发交接（2026-10-24）

当前为 Windows x64 的 MarkdownReader 0.1.0 Preview，使用 Go 1.26.1、Wails v2、Vditor，固定 WebView2 155.0.4283.45 和 Pandoc 3.12.1。

用户当前优先级：保持已有功能稳定、准备首版源码；暂缓详细 Typora 差异对比和大量 GUI 测试。完整功能对齐目标尚未完成，已知限制见 [STATUS.md](STATUS.md)。

清理前的稳定源码已通过前端 77 项、解析器 3 项、Go 全套测试、go vet 和生产构建；便携 ZIP 已检查必要文件和 EXE 哈希。这是本机验证记录，不代表干净 Windows 环境验收。

保留锁文件、`scripts/`、`build/windows/`、`third_party/` 和许可证；其中含定制 WebView2 依赖、Lute 校验及补丁，不应随意换成上游最新版。源码内不应写入开发者本机目录；个人数据和缓存保持在忽略目录。

近期系统拼写接口实验未合入：本机只读测试曾观察到 `quikc → quick/quack`、`colour → color/cooler`，英式模式没有得到预期差异。这不代表应用已实现系统拼写，也不能推广到其他 Windows。完整 Hunspell 实验未解决相同词典的候选问题。当前保留已验证的内置美式/英式词典和个人词典实现；恢复工程不依赖丢弃的实验二进制或截图。

恢复操作见根目录 [RECOVERY.md](../RECOVERY.md)。第三方许可已保留，项目自身许可证尚待维护者选择。此次未推送 GitHub。
