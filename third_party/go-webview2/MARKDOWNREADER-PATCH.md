# MarkdownReader 本地兼容补丁

基于 github.com/wailsapp/go-webview2 v1.0.22，包含两项本地改动：

1. pkg/edge/chromium.go将进程级MARKDOWNREADER_BROWSER_ARGUMENTS追加到传入WebView2 loader的参数。
2. 同文件的WebMessageReceived转入pkg/edge/markdownreader_devtools.go，处理MarkdownReader自己的开发者工具请求。在WebView2 COM/UI线程调用GetSettings、启用该视图的检查器并调用OpenDevToolsWindow，返回带请求ID的真实成功/错误结果。双重核对顶层和发送方来源为http://wails.localhost，其他来源不执行。普通Wails消息仍走原有回调和回送。

原因：loader 会替换 WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS，Wails v2 没有公开自定义浏览器参数接口。MarkdownReader 使用绑定 127.0.0.1 的随机端口连接本应用页面，调用 Page.printToPDF / Page.captureScreenshot 完成离线 PDF/PNG 导出。

原始上游许可证保留。本目录通过 go.mod replace 引用，构建不需要修改全局模块缓存。

开发者工具来源校验及本包原有测试通过 `go test ./pkg/edge` 执行。原生显示、Shift+F12、重复打开、关闭后文档/历史保持及既有离线PDF/公式回归结果记录在主项目docs中。
