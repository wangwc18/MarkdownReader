# Lute 本地兼容补丁

上游：Vditor 4.0.0 随包的 dist/js/lute/lute.min.js。
原始 SHA256：53b61b051a3dfd9ee5a5010c873233679e139d8e62c49da08b4c2f2c195587cb。

2026-10-19，MarkdownReader 对生成的 JavaScript 资源增加三个公式解析修复：起始 $ 后不能紧跟空格/制表符/换行，结束 $ 前不能是空格/制表符；禁用数字开头公式时，将含成对分隔符、内容为不含空白的 ASCII 数字公式作为一个完整字面片段处理，避免其结束符抢占后续公式。空白与非 ASCII 的数字候选仍沿用其他已有解析路径，不将该局部修复宣称为完整数学语法对齐。

保留上游版权和许可证，不修改 node_modules 中的原始依赖。空白规则参考 https://support.typora.io/Math/ ，仍需本机 Typora 1.3.8 实测确认版本差异。

scripts/prepare-assets.ps1 复制资源后调用 scripts/patch-lute.mjs。脚本验证完整上游 SHA256、替换锚点与数字分支结构，依赖变化即终止构建，不静默跳过补丁。运行脚本会报告补丁后 SHA256。生产前端构建从 public 复制修复资源，Go 将最终前端 dist 嵌入 EXE。

这不是 Markdown 原文预处理：源码不插入转义字符、不替换用户公式，不改变公式块解析；同一 Lute 资源供即时渲染与 HTML 导出使用。源码包包含应用补丁所需脚本、固定依赖锁和本文；便携包包含已打补丁资源及原许可证。

## 2026-10-20：严格/宽松解析

对照本机 Typora 1.3.8 基线及上游 v1.7.6 的 parse/heading.go、parse/list.go，在同一固定资源中增加实例级 MarkdownReaderLoose 选项：宽松时接受 # 后无空白的 ATX 标题，并允许列表按两列最小缩进继续；严格时保留原标记宽度规则。标题标记切片单独处理无空白情况，避免将正文第一个字纳入标记。

原 GopherJS 包装器有 `__internal_object__.ParseOptions` 属性。脚本为固定版本的 Lute.New 增加检查过的桥接，暴露 SetMarkdownReaderStrictMode；默认严格，新实例读取应用配置。选项保存在每个实例内部，不通过修改 Markdown 字符串实现。桥接或替换锚点变化时构建失败。

当前补丁后 SHA256：12467112bd0c515490c7e2c46c2b21bebdf1dc11b800a6afeec5033ec56b6874。上游资源哈希不变。scripts/lute-modes.test.mjs 已接入构建脚本，检查标题/字面量、列表容器、实例隔离和默认设置；桌面结果见 strict-mode-smoke-result.json、strict-save-roundtrip-result.json。所测样本不等同于全部 GFM 或全部编辑交互对齐。

源代码检查依据：[上游 v1.7.6 标题解析](https://github.com/88250/lute/blob/v1.7.6/parse/heading.go)、[列表解析](https://github.com/88250/lute/blob/v1.7.6/parse/list.go)。官方标签源码 ZIP 检查用副本 SHA256 为 7EB30AE5F99D50A4928F54BF7B1806F4E3EF0B04F3F242AA59CFE2CC41BA83B7；该研究副本未作为额外运行时依赖分发。
