# Typora 1.3.8 编码菜单基线

2026-10-16，computer-use 实际读取“文件 → 选择编码重新打开”的完整列表，并通过原生打开对话框打开 `examples/编码示例/GBK 中文.md`。Typora 显示标题“中文 GBK”和正文“中文编码保存往返。”，未对文件执行保存。

菜单包含：自动、UTF-8、UTF-16 LE、UTF-16 BE；Windows-1252/1251/1250/1253/1255；ISO-8859-2/7/8；IBM866/855；KOI8-R；MacCyrillic；GB2312、GB18030、Big5、SHIFT_JIS、EUC-JP、EUC-KR、TIS-620。

本机 Typora 还把“Cyrillic”与“ISO-8859-2”组合显示，并将 Greek 显示为 Geek；MarkdownReader 使用明确的 ISO-8859-5（西里尔）与 ISO-8859-2（中欧）名称，不复制这两处标签。

MarkdownReader 提供 27 个具体编码选项，额外包含 GBK、UTF-8 BOM 及无 BOM 的 UTF-16 LE/BE。自动识别优先 Unicode BOM/UTF-8，再处理明显的无 BOM UTF-16 和合法 GB18030；不能视为 Typora 自动识别算法的完全复现。其他编码可显式选择“按编码打开”或“选择编码重新打开”。

“保存编码”为补充功能：保留当前文本，验证目标编码可表示所有字符后标记待保存。重新打开本身不会转换磁盘字节，未编辑保存也不重写原文件。重新解码前的未保存确认、取消保留、无损失败保护和字节往返已有测试，所有编码边界与自动探测仍需继续对比。
