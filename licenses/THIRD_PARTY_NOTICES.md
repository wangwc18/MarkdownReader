# 第三方组件

主程序使用 Wails v2.16.0、go-webview2 v1.0.22（本地参数传递补丁）、Vditor 4.0.0、diff-match-patch 1.0.5。Go 组件许可证保存在 `Go/`，JavaScript 主组件许可证保存在本目录。

英文拼写使用 nspell 2.1.5（MIT）、dictionary-en 4.0.0 与 dictionary-en-gb 3.0.0（MIT AND BSD），以及 is-buffer 2.0.5（MIT）。对应许可为 Nspell-LICENSE.txt、EnglishDictionary-LICENSE.txt、BritishDictionary-LICENSE.txt 和 IsBuffer-LICENSE.txt；版本由前端锁文件固定。Hunspell WebAssembly 与 Windows 系统拼写的研究实现未纳入本次首版。

导出元数据使用 YAML 官方维护的 go.yaml.in/yaml/v3 v3.0.5，来源 https://github.com/yaml/go-yaml 。其 LICENSE 和 NOTICE 分别保存在 Go/go.yaml.in-yaml-v3-LICENSE.txt、Go/go.yaml.in-yaml-v3-NOTICE.txt，版本与完整性由 go.mod/go.sum 锁定。

Vditor 随带的资源包括 Lute、KaTeX/mhchem、MathJax、highlight.js、Mermaid、flowchart.js、ABCJS、ECharts、Markmap、WaveDrom、Smiles Drawer、PlantUML Encoder 和 Viz.js。各文件原始版权头保持完整，资源哈希记录用于锁定随包版本。

本机运行核对随包 MathJax 为 3.1.2；physics 扩展使用同一资源树内的 input/tex/extensions/physics.js，2026-10-23 新增应用配置开关联动，没有修改 MathJax 分发文件。许可见 MathJax-LICENSE.txt。

2026-10-19，MarkdownReader 对固定 Lute JavaScript 资源加入行内公式空白边界与数字字面量兼容修复。具体修改、原始 SHA256 和可重复应用脚本见 docs/lute-patches.md 与 scripts/patch-lute.mjs；保留 Lute-LICENSE.txt 和上游版权头。

2026-10-20，同一固定资源加入标题和列表的严格/宽松解析选项及固定 GopherJS 实例桥接。修改说明、补丁哈希和对应测试见 docs/lute-patches.md；原始资源及许可证保持不变。

Viz.js 2.1.2 随带 Graphviz 2.40.1、Expat 2.2.5、zlib 与 Emscripten 生成代码；原文件头注明其对应许可。Pandoc 3.12.1 的版权列表及许可位于 `Pandoc-COPYRIGHT.txt`、`Pandoc-COPYING.rtf`；对应源码发布页：https://github.com/jgm/pandoc/releases/tag/3.12.1 。

WebView2 固定运行时 155.0.4283.45 x64 来自 Microsoft 官方下载，并保留运行时自带的第三方软件许可文件。官方分发说明：https://learn.microsoft.com/en-us/microsoft-edge/webview2/concepts/distribution 。

许可证清单仍需要最终检查后才可作为正式发布版本；本包用于本地开发验证。
