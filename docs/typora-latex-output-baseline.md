# 无格式 HTML 的 LaTeX 输出基线（2026-10-23）

通过 computer-use 操作本机 Typora 1.3.8：偏好设置 → Markdown → 当复制或导出为无格式的 HTML 时，将“使用 SVG”临时切换为“使用 LaTeX 代码”。关闭偏好页，通过文件 → 导出 → HTML (without styles) 将当前《公式编号验证.md》导出到工作区 build/test-tools/typora-latex-immediate.html。

此操作没有重启；实际文件已采用 LaTeX 输出。不能据偏好页整体“重启生效”提示推断该项必须重启，也不推断其他选项生效时机。读取结果后，重新打开偏好设置恢复“使用 SVG”，可访问性与截图均确认恢复，关闭偏好页。未编辑或保存原 Markdown。

实际文件副本见 typora-latex-output.html：行内保留 `$q=1$`、`$\ref{last}$` 等原始包裹形式；五个独立公式分别为 `<div>$$`、换行、TeX、换行、`$$</div>`。align 的 `&` 在 HTML 文件中为 `&amp;`，解析后仍是原始 TeX。没有公式 SVG、MathJax 脚本或预计算编号。

本轮固定样本仅证明此导出选项的上述结果。原生选区复制、特殊字符、复杂嵌套容器及 SVG 输出的完整 HTML 结构仍待逐项比较；MarkdownReader 的选区和字面代码检查是自身产品验证，不能替代这些原生基线。
