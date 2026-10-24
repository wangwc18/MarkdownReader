---
title: 项目周报
author: 作者甲
project:
  name: MarkdownReader
  version: 0.1
people: [作者甲, 作者乙]
---

# YAML 导出变量示例

在自定义 Pandoc 导出项中设置以下附加参数，输出格式使用 `html5`、扩展名使用 `html`：

```text
--metadata=title:${project.name} / ${title}
--metadata=author:${people.0}
```

输出 HTML 的标题应为“MarkdownReader / 项目周报”，作者应为“作者甲”。转换使用当前编辑内容；修改前言后可以直接导出，不必先保存 Markdown。
