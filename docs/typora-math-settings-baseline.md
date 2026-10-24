# Typora 1.3.8 公式设置（2026-10-23）

computer-use 打开本机偏好设置 → Markdown，滚动到公式区，截图核对：

| 项目 | 当前状态 | 验收状态 |
| --- | --- | --- |
| 自动添加序号 | 未勾选 | 开关行为与编号范围待测 |
| 支持使用 `\\` 和 `\newline` 换行 | 勾选 | 复杂环境和关闭行为待测 |
| 启用 physics 包 | 未勾选 | 开/关渲染对照待测 |
| 当复制或导出为无格式 HTML 时 | 使用 SVG | 输出细节待测 |
| 上一项的另一个选项 | 使用 LaTeX 代码 | 实际输出待测 |

页面提示这些选项在重启后生效。只打开下拉读取两个选项，未改变本机设置。新增《公式扩展验证.md》作为后续固定对照内容，尚不能将其全部语法计为原生验证通过。

现有应用普通预览为 KaTeX，无格式 HTML 为本地 MathJax SVG；MathJax 与 physics 扩展文件已随包，但先前没有配置开关和转换联动。增加资源存在检查或成功渲染少数命令，不能视为完整 physics 包和 Typora 行为已对齐。

实现依据：MathJax 3 系列官方说明可在修改配置后使用 startup.getComponents 重新建立输入/输出组件，避免只改全局配置但已有输入器不更新。实际随包运行版本为3.1.2，已在桌面验证此接口和本地physics扩展加载。

- https://docs.mathjax.org/en/v3.2/web/configuration.html
- https://docs.mathjax.org/en/v3.1/web/configuration.html
- https://support.typora.io/Math/ （当前网页更新晚于本机版本，功能基线以本机实际观察为准）

## 本轮实现与验证范围

新增physics开关，默认关闭；开启时编辑预览和HTML使用本地MathJax，关闭时普通预览恢复KaTeX。SVG导出使用同一开关重建MathJax输入组件，防止先做过一次导出后开关失效。源码模式保存设置后切回预览也更新引擎。

`physics-smoke.js` 27项实际桌面检查通过：先SVG导出、开启/关闭/重新开启、导数和偏导及bra/ket样本、HTML和无样式HTML文件生成与解析、关闭时未知导数命令的红色输出、开启后无该错误、原文和模式往返、后端设置及本地脚本来源。原生computer-use勾选开关、保存、切换模式，看到df/dx；随后3项状态断言通过并恢复关闭，记录 `physics-cua-result.json`，实际MathJax版本3.1.2。

初轮检查失败发现源码模式保存设置后仍沿用旧引擎，已修复并复测；bra/ket在KaTeX本来就有部分支持，测试不再错误要求其关闭时必须报错。以上不代表Typora开启physics并重启后的完整对照，也不代表所有physics命令、自动编号、换行选项和LaTeX输出已完成。

同日后续加入公式换行偏好后，普通公式也统一改用MathJax，关闭physics不再切换回KaTeX。既有physics检查改为检查MathJax未知命令的红色输出，并完整复测27项通过。换行基础样本的原生开启/关闭重启对照及60项桌面检查见typora-math-line-breaks-baseline.md；复杂语法和编号/LaTeX输出仍待验收。
