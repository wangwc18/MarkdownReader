import Vditor from "vditor";
import DiffMatchPatch from "diff-match-patch";
import { lineBounds, sentenceBounds, replaceMatches, replacementText } from "./text-editing.js";
import { markdownTables, tableAt, transformTable } from "./table-editing.js";
import { recordHistory as addHistory } from "./history.js";
import { markdownLinks, linkAt, writeLink } from "./markdown-links.js";
import { restoreMediaReferences } from "./media-references.js";
import { restoreDataSVGImages } from "./data-image-render.js";
import { searchPattern } from "./search-pattern.js";
import { imageMoveAliases, movedImageReferences } from "./image-move-references.js";
import {imageInsertPresets, selectedImageInsertPreset, imageInsertPresetSettings} from "./image-insert-presets.js";
import {PunctuationView} from './render-punctuation.js';
import {englishWords,spellingTextNodes,isPersonalWord,effectiveSpelling} from './offline-spelling.js';
import { clearSourceSearchHighlights, paintSourceSearchHighlights, syncSourceSearchScroll } from "./source-search-highlight.js";
import { formatRange } from "./format-selection.js";
import { scrollSourceSelection, scrollRenderedSelection } from "./selection-scroll.js";
import { decorateBreakMarkers } from "./br-markers.js";
import { canConvertSmartDash } from "./smart-punctuation.js";
import { serializeReferenceImages } from "./reference-image-serialization.js";
import { preserveRenderedSource } from "./rendered-source.js";
import { documentStatistics } from "./document-statistics.js";
import { prepareMathJax, renderDocumentMath } from "./math-engine.js";
import { taskStates, pureTaskListAt } from "./task-editing.js";
import { openDeveloperTools } from "./devtools.js";
import { documentImages, scaledImage, imageScales, replaceImageDestination, replaceImageDestinations } from "./image-scaling.js";
import "vditor/dist/index.css";
import "./style.css";

const $ = (s) => document.querySelector(s),
  esc = (s) =>
    String(s ?? "").replace(
      /[&<>"']/g,
      (c) =>
        ({
          "&": "&amp;",
          "<": "&lt;",
          ">": "&gt;",
          '"': "&quot;",
          "'": "&#39;",
        })[c],
    );
const cdn = "/vendor/vditor";
const diff = new DiffMatchPatch();
let statisticsSelection = null;
let mathObserver = null, mathRenderTimer;
let api,
  settings,
  editor,
  docs = [],
  active = null,
  source = false,
  loading = false,
  ready = false,
  side = "outline",
  showSide = false,
  folder = "",
  tree = [],
  recovery = [],
  revision = 0,
  syncTimer,
  saveTimer,
  zoom = 1,
  topmost = false,
  fullscreen = false,
  lastRange = null,
  findPos = -1;
let syncChain = Promise.resolve();
let exportInProgress = false;
let pendingModal = null;
let pendingInput = null, compositionCounter = 0, currentComposition = null;
let lastLinkContext = null;
let historyNavigation = Promise.resolve();
let encodingOptions = [];
let settingsBaseline;
let settingsSyncBusy = false, settingsSaveBusy = false, lastInputAt = 0;
let pasteSerial = 0;
let renderedBaseline = null;
function acceptSettings(value) {
  settings = value;
  settingsBaseline = structuredClone(value);
}
const exportedAssetCache = new Map();
const app = $("#app");
app.innerHTML = `<header id="menubar" aria-label="主菜单"></header><div id="tabs"></div><section id="findbar" hidden><input id="find-query" placeholder="查找" aria-label="查找文本"><input id="replace-query" placeholder="替换为" aria-label="替换文本"><label><input id="find-case" type="checkbox">区分大小写</label><label><input id="find-regex" type="checkbox">正则</label><label><input id="find-word" type="checkbox">全词匹配</label><button id="find-prev">上一个</button><button id="find-next">下一个</button><button id="replace-one">替换</button><button id="replace-all">全部替换</button><span id="find-result"></span><button id="find-close">×</button></section><div id="conflict" hidden>文件已被外部修改。<button id="reload">重新加载</button><button id="conflict-saveas">另存为</button><button id="conflict-dismiss">稍后处理</button></div><main><aside id="sidebar" hidden><nav><button data-side="outline">大纲</button><button data-side="list">文档</button><button data-side="tree">文件</button><button data-side="search">搜索</button></nav><div id="side-content"></div></aside><section id="work"><div id="editor"></div><textarea id="source" spellcheck="false" hidden aria-label="Markdown 源代码编辑器"></textarea></section></main><footer id="status"><div><button id="sidebar-toggle" title="侧边栏 Ctrl+Shift+L">☰</button><button id="source-toggle" title="源码 Ctrl+/">&lt;/&gt;</button><span id="mode">即时渲染</span><span id="file-state"></span></div><button id="count" title="文档统计">0 字</button></footer><dialog id="modal"><form id="modal-form"><h2 id="modal-title"></h2><div id="modal-content"></div><div id="modal-actions"></div></form></dialog><div id="toast" role="status" hidden></div><article id="print-area" class="vditor-reset" hidden></article>`;

document.body.append($("#print-area"));

function notify(message, error = false) {
  const t = $("#toast");
  t.textContent = String(message);
  t.dataset.error = error;
  t.hidden = false;
  clearTimeout(t.timer);
  t.timer = setTimeout(() => (t.hidden = true), error ? 8000 : 3500);
  if (error && api) api.LogError(String(message)).catch(() => {});
}
async function writeClipboard(text) {
  if (!(await window.runtime.ClipboardSetText(text))) throw Error("无法写入 Windows 剪贴板");
}
async function safely(fn) {
  try {
    return await fn();
  } catch (e) {
    notify(e?.message || e, true);
    return null;
  }
}
window.addEventListener("error", (e) => notify(e.message, true));
window.addEventListener("unhandledrejection", (e) =>
  notify(e.reason?.message || e.reason, true),
);
function dialog(title, html, buttons = [["确定", "ok"]]) {
  return new Promise((resolve) => {
    if (pendingModal) pendingModal({ value: "cancel", data: {} });
    const finish = (result) => {
      pendingModal = null;
      resolve(result);
    };
    pendingModal = finish;
    const m = $("#modal");
    if (m.open) m.close();
    $("#modal-title").textContent = title;
    $("#modal-content").innerHTML = html;
    $("#modal-actions").innerHTML = "";
    for (const [label, value] of buttons) {
      const b = document.createElement("button");
      b.type = "button";
      b.textContent = label;
      b.onclick = () => {
        const data = Object.fromEntries(
          new FormData($("#modal-form")).entries(),
        );
        m.close();
        finish({ value, data });
      };
      $("#modal-actions").append(b);
    }
    m.oncancel = (e) => {
      e.preventDefault();
      m.close();
      finish({ value: "cancel", data: {} });
    };
    $("#modal-form").onsubmit = (e) => e.preventDefault();
    m.showModal();
    const field = m.querySelector("input:not([type=checkbox]),textarea,select");
    if (field) field.focus();
  });
}
function closeDialog() {
  $("#modal").close();
  if (pendingModal) pendingModal({ value: "cancel", data: {} });
}
async function promptText(title, label, value = "") {
  const r = await dialog(
    title,
    `<label>${esc(label)}<input name="text" value="${esc(value)}" autocomplete="off"></label>`,
    [
      ["确定", "ok"],
      ["取消", "cancel"],
    ],
  );
  return r.value === "ok" ? r.data.text : null;
}
const cmd = (name, id, key = "", children) => ({ name, id, key, children });
const exportItems = [
  ["PDF", "pdf"],
  ["HTML", "html"],
  ["HTML（无样式）", "html-plain"],
  ["图像 PNG", "png"],
  ["Word (.docx)", "docx"],
  ["OpenOffice", "odt"],
  ["RTF", "rtf"],
  ["Epub", "epub"],
  ["LaTeX", "tex"],
  ["Media Wiki", "wiki"],
  ["reStructuredText", "rst"],
  ["Textile", "textile"],
  ["OPML", "opml"],
].map(([n, f]) => cmd(n, "export:" + f));
const menus = [
  [
    "文件",
    [
      cmd("新建", "new", "Ctrl+N"),
      cmd("新建窗口", "window", "Ctrl+Shift+N"),
      null,
      cmd("打开…", "open", "Ctrl+O"),
      cmd("打开文件夹…", "folder"),
      cmd("快速打开…", "quick", "Ctrl+P"),
      cmd("打开最近文件", "recent", "", []),
      cmd("按编码打开…", "openencoding"),
      cmd("选择编码重新打开", "reopen-menu", "", []),
      null,
      cmd("保存", "save", "Ctrl+S"),
      cmd("另存为…", "saveas", "Ctrl+Shift+S"),
      cmd("保存全部打开的文件", "saveall"),
      cmd("保存编码", "saveencoding-menu", "", []),
      null,
      cmd("属性…", "properties"),
      cmd("打开文件位置", "reveal"),
      cmd("在侧边栏中显示", "showfile"),
      cmd("重命名…", "renamefile"),
      cmd("删除…", "deletefile"),
      cmd("恢复删除的文件…", "restorefile"),
      null,
      cmd("导入…", "import"),
      cmd(
        "导出",
        "export",
        "",
        exportItems.concat([
          null,
          cmd("使用上一次设置导出", "exportlast", "Ctrl+Shift+E"),
          cmd("导出并覆盖上一次文件", "exportoverwrite"),
          cmd("导出设置…", "preferences:export"),
        ]),
      ),
      cmd("打印…", "print", "Alt+Shift+P"),
      null,
      cmd("偏好设置…", "preferences", "Ctrl+,"),
      cmd("关闭文档", "close", "Ctrl+W"),
    ],
  ],
  [
    "编辑",
    [
      cmd("撤销", "undo", "Ctrl+Z"),
      cmd("重做", "redo", "Ctrl+Y"),
      null,
      cmd("剪切", "cut", "Ctrl+X"),
      cmd("复制", "copy", "Ctrl+C"),
      cmd("粘贴", "paste", "Ctrl+V"),
      null,
      cmd("复制为纯文本", "copytext"),
      cmd("复制为 Markdown", "copymd", "Ctrl+Shift+C"),
      cmd("复制为 HTML 代码", "copyhtml"),
      cmd("复制内容并简化格式", "copysimple"),
      cmd("粘贴为纯文本", "pastetext", "Ctrl+Shift+V"),
      null,
      cmd("选择", "select", "", [
        cmd("全选", "selectall", "Ctrl+A"),
        cmd("选择段落或块", "selectblock", "Alt+Ctrl+P"),
        cmd("选中当前行或句", "selectline", "Ctrl+L"),
        cmd("选中当前格式文本", "selectformat", "Ctrl+E"),
        cmd("选中当前词", "selectword", "Ctrl+D"),
        cmd("跳转到文首", "start", "Ctrl+Home"),
        cmd("跳转到所选内容", "scrollselection", "Ctrl+J"),
        cmd("跳转到文末", "end", "Ctrl+End"),
      ]),
      cmd("删除", "delete"),
      cmd("删除范围", "delete-range", "", [
        cmd("删除块", "deleteblock", "Alt+Ctrl+Shift+P"),
        cmd("删除当前行", "deleteline", "Alt+Ctrl+Shift+L"),
        cmd("删除当前格式文本", "deleteformat", "Alt+Ctrl+Shift+E"),
        cmd("删除当前词", "deleteword", "Ctrl+Shift+D"),
      ]),
      cmd("数学工具", "math-tools", "", [cmd("刷新所有数学公式", "math-refresh")]),
      cmd("换行符", "eol", "", [
        cmd("Windows 换行符 (CRLF)", "eol:CRLF"),
        cmd("Unix 换行符 (LF)", "eol:LF"),
        cmd("保存时在文末添加空行", "final-newline"),
      ]),
      cmd("空格与换行", "whitespace", "", [
        cmd("首行缩进", "paragraph-indent"),
        cmd("显示 <br/>", "show-br-markers"),
        cmd("保留单换行符", "keep-single-line-breaks"),
        cmd("删除行尾空格", "trimspaces"),
      ]),
      cmd("拼写检查…", "spell-language"),
      null,
      cmd("查找…", "find", "Ctrl+F"),
      cmd("查找下一个", "findnext", "F3"),
      cmd("查找上一个", "findprev", "Shift+F3"),
      cmd("替换…", "replace", "Ctrl+H"),
      cmd("表情与符号", "emoji"),
    ],
  ],
  [
    "段落",
    [
      ...Array.from({ length: 6 }, (_, i) =>
        cmd(
          `${["一", "二", "三", "四", "五", "六"][i]}级标题`,
          "heading:" + (i + 1),
          "Ctrl+" + (i + 1),
        ),
      ),
      cmd("段落", "heading:0", "Ctrl+0"),
      null,
      cmd("提升标题级别", "heading-up", "Ctrl+="),
      cmd("降低标题级别", "heading-down", "Ctrl+-"),
      null,
      cmd("表格", "table-menu", "", [
        cmd("插入表格…", "table", "Ctrl+T"), null,
        cmd("上方插入行", "table:row-above"),
        cmd("下方插入行", "table:row-below", "Ctrl+Enter"), null,
        cmd("左侧插入列", "table:column-left"),
        cmd("右侧插入列", "table:column-right"), null,
        cmd("上移该行", "table:row-up", "Alt+ArrowUp"),
        cmd("下移该行", "table:row-down", "Alt+ArrowDown"),
        cmd("左移该列", "table:column-leftward", "Alt+ArrowLeft"),
        cmd("右移该列", "table:column-rightward", "Alt+ArrowRight"), null,
        cmd("删除行", "table:delete-row", "Ctrl+Shift+Backspace"),
        cmd("删除列", "table:delete-column"), null,
        cmd("复制表格", "table:copy"),
        cmd("格式化表格源码", "table:format"),
        cmd("调整表格…", "table:resize"),
        cmd("列对齐", "table-align", "", [cmd("左对齐", "table:align-left"),cmd("居中对齐", "table:align-center"),cmd("右对齐", "table:align-right")]),
        null, cmd("删除表格", "table:delete-table"),
      ]),
      cmd("代码块", "codeblock", "Ctrl+Shift+K"),
      cmd("公式块", "mathblock", "Ctrl+Shift+M"),
      cmd("引用", "quote", "Ctrl+Shift+Q"),
      cmd("有序列表", "ordered-list", "Ctrl+Shift+["),
      cmd("无序列表", "list", "Ctrl+Shift+]"),
      cmd("任务列表", "check", "Ctrl+Shift+X"),
      cmd("任务状态", "task-status", "", [cmd("切换任务状态", "task-toggle"),null,cmd("标记为已完成", "task-complete"),cmd("标记为未完成", "task-incomplete")]),
      cmd("列表缩进", "list-indent", "", [
        cmd("增加缩进", "indent", "Ctrl+]"),
        cmd("减少缩进", "outdent", "Ctrl+["),
      ]),
      null,
      cmd("在上方插入段落", "insertbefore"),
      cmd("在下方插入段落", "insertafter"),
      cmd("链接引用", "reference"),
      cmd("脚注", "footnote"),
      cmd("水平分割线", "line"),
      cmd("内容目录", "toc"),
      cmd("YAML Front Matter", "yaml"),
    ],
  ],
  [
    "格式",
    [
      cmd("加粗", "bold", "Ctrl+B"),
      cmd("斜体", "italic", "Ctrl+I"),
      cmd("下划线", "underline", "Ctrl+U"),
      cmd("代码", "inline-code", "Ctrl+Shift+`"),
      cmd("内联公式", "inlinemath"),
      cmd("删除线", "strike", "Alt+Shift+5"),
      cmd("高亮", "mark"),
      cmd("上标", "sup"),
      cmd("下标", "sub"),
      cmd("注释", "comment"),
      null,
      cmd("超链接", "link", "Ctrl+K"),
      cmd("链接操作", "link-menu", "", [cmd("打开链接", "link:open"),cmd("复制链接地址", "link:copy"),cmd("编辑链接…", "link:edit"),cmd("移除链接", "link:remove")]),
      cmd("图像", "image-menu", "", [
        cmd("插入图片…", "imageurl", "Ctrl+Shift+I"),cmd("插入本地图片…", "image"),null,
        cmd("编辑图片地址…", "image:edit"),cmd("打开图片位置…", "image:reveal"),cmd("拷贝图片", "image:bitmap"),cmd("复制图片地址", "image:copy"),
        cmd("缩放图片", "image-scale", "", imageScales.flatMap(n=>[...(n===100||n===150?[null]:[]),cmd(n+"%", "image-scale:"+n)])),
        cmd("复制图片到…", "image-file-copy"),
        cmd("移动图片到…", "image-file-move"),
        cmd("复制所有图片到…", "image-files-copy"),
        cmd("移动所有图片到…", "image-files-move"),
        cmd("上传图片", "image-upload"),cmd("上传所有本地图片", "image-upload-all"),
        cmd("重新加载所有图片", "imagesreload"),cmd("当插入本地图片时", "image-insert-rules", "", [cmd("复制图片到文件夹…", "image-copy-folder"),cmd("上传图片", "image-upload-rule")]),cmd("设置图片根目录…", "image-root"),cmd("全局图像设置…", "preferences:image"),
      ]),
      cmd("清除样式", "clearstyle", "Ctrl+\\"),
    ],
  ],
  [
    "视图",
    [
      cmd("显示／隐藏侧边栏", "sidebar", "Ctrl+Shift+L"),
      cmd("大纲", "side:outline", "Ctrl+Shift+1"),
      cmd("文档列表", "side:list", "Ctrl+Shift+2"),
      cmd("文件树", "side:tree", "Ctrl+Shift+3"),
      cmd("搜索", "side:search", "Ctrl+Shift+F"),
      null,
      cmd("源代码模式", "source", "Ctrl+/"),
      cmd("专注模式", "focus", "F8"),
      cmd("打字机模式", "typewriter", "F9"),
      cmd("显示状态栏", "status"),
      cmd("切换全屏", "fullscreen", "F11"),
      cmd("保持窗口在最前端", "topmost"),
      null,
      cmd("实际大小", "zoomreset", "Ctrl+Shift+9"),
      cmd("放大", "zoomin", "Ctrl+Shift+="),
      cmd("缩小", "zoomout", "Ctrl+Shift+-"),
      cmd("切换窗口", "nextwindow", "Ctrl+Tab"),
      cmd("切换到上一个窗口", "prevwindow", "Ctrl+Shift+Tab"),
      cmd("窗口列表…", "windows"),
      cmd("切换文档", "nextdoc", "Ctrl+PageDown"),
      null,cmd("开发者工具", "devtools", "Shift+F12"),
    ],
  ],
  [
    "主题",
    ["Github", "Newsprint", "Night", "Pixyll", "Whitey"]
      .map((t) => cmd(t, "theme:" + t))
      .concat([null, cmd("自定义 CSS…", "customcss")]),
  ],
  ["帮助", [cmd("使用说明", "help"), cmd("关于 MarkdownReader", "about")]],
];
const keys = new Map();
const sourceDisabledCommands = new Set();
function collectCommandIDs(items, target) {
  for(const item of items){if(!item)continue;target.add(item.id);if(item.children)collectCommandIDs(item.children,target);}
}
for(const [name,items] of menus)if(name==='段落'||name==='格式')collectCommandIDs(items,sourceDisabledCommands);
function taskListContext(){
  if(source)return null;
  const root=$('#editor .vditor-ir > .vditor-reset'),selection=getSelection();
  const live=selection?.rangeCount?selection.getRangeAt(0):null;
  const range=live&&root?.contains(live.startContainer)&&root.contains(live.endContainer)?live:lastRange;
  if(!range||!root?.contains(range.startContainer)||!root.contains(range.endContainer))return null;
  const element=range.startContainer.nodeType===1?range.startContainer:range.startContainer.parentElement;
  if(element.closest('pre:not(.vditor-reset),[data-type="code-block"],table'))return null;
  const li=element.closest('li')||[...root.querySelectorAll('li')].find(node=>range.intersectsNode(node));
  if(!li||![...li.parentElement.children].every(node=>node.tagName==='LI'&&node.classList.contains('vditor-task')))return null;
  if(!pureTaskListAt(content(),markdownSelection()[0]))return null;
  if([...root.children].some(node=>range.intersectsNode(node)&&!node.contains(li)&&!node.querySelector('li.vditor-task')))return null;
  return {li,range};
}
function commandEnabled(id){
  if(source&&sourceDisabledCommands.has(id))return false;
  if(id==='image-scale'||id.startsWith('image-scale:')||id==='image-file-copy')return !!active&&!!currentLink(true);
  if(id==='image-file-move'){const image=active&&currentLink(true);return !!image&&!/^(https?:|data:|blob:)/i.test(image.destination);}
  if(id==='image-files-copy')return !!active&&documentImages(content()).length>0;
  if(id==='image-files-move')return !!active&&documentImages(content()).some(image=>!/^(https?:|data:|blob:)/i.test(image.destination));
  if(id==='image-upload')return !!active&&!!currentLink(true);
  if(id==='image-upload-all')return !!active&&documentImages(content()).some(image=>! /^(https?:|data:|blob:)/i.test(image.destination));
  if(['task-status','task-toggle','task-complete','task-incomplete'].includes(id))return !!taskListContext();
  return true;
}
function refreshMenuAvailability(){
  for(const row of document.querySelectorAll('#menubar [data-command]')){
    const enabled=commandEnabled(row.dataset.command);row.setAttribute('aria-disabled',String(!enabled));row.tabIndex=enabled?0:-1;
  }
}
function keyNames(items) {
  for (const item of items) {
    if (!item) continue;
    if (item.key) keys.set(item.key.toLowerCase(), item.id);
    if (item.children) keyNames(item.children);
  }
}
menus.forEach(([, items]) => keyNames(items));
function menuEntries(items) {
  const ul = document.createElement("div");
  ul.className = "dropdown";
  ul.setAttribute("role", "menu");
  for (const item of items) {
    if (!item) {
      ul.append(document.createElement("hr"));
      continue;
    }
    const row = document.createElement("div");
    row.className = "menuitem";
    row.setAttribute("role", "menuitem");
    row.dataset.command=item.id;
    row.tabIndex = 0;
    const label = document.createElement("span");
    label.textContent = item.name;
    row.append(label);
    const k = document.createElement("small");
    k.textContent = item.children ? "›" : item.key;
    row.append(k);
    row.onmousedown = (e) => e.preventDefault();
    if (item.children) {
      row.classList.add("submenu");
      const submenu = menuEntries(item.children);
      row.append(submenu);
      const position = () => {
        if(!commandEnabled(item.id))return;
        const anchor = row.getBoundingClientRect();
        submenu.style.left = Math.max(4, Math.min(anchor.right - 3, innerWidth - submenu.offsetWidth - 4)) + "px";
        submenu.style.top = Math.max(4, Math.min(anchor.top, innerHeight - submenu.offsetHeight - 4)) + "px";
      };
      row.onmouseenter = position;
      row.onfocus = position;
    } else {
      row.onclick = () => {
        if(!commandEnabled(item.id))return;
        closeMenus();
        safely(() => run(item.id));
      };
      row.onkeydown = (e) => {
        if (e.key === "Enter") {
          if(!commandEnabled(item.id))return;
          closeMenus();
          safely(() => run(item.id));
        }
      };
    }
    ul.append(row);
  }
  return ul;
}
function buildMenus() {
  menus[0][1].find(item=>item?.id==='export').children = exportItems.concat(
    (settings?.customExports?.length ? [null,...settings.customExports.map(preset=>cmd(preset.name,'export:custom:'+preset.id))] : []),
    [null,cmd('使用上一次设置导出','exportlast','Ctrl+Shift+E'),cmd('导出并覆盖上一次文件','exportoverwrite'),cmd('导出设置…','preferences:export')]);
  const bar = $("#menubar");
  bar.innerHTML = "";
  for (const [name, items] of menus) {
    const wrap = document.createElement("div");
    wrap.className = "menu";
    const b = document.createElement("button");
    b.textContent = name;
    b.setAttribute("aria-haspopup", "menu");
    b.onmousedown = (e) => {
      e.preventDefault();
      rememberSelection();
    };
    b.onclick = (e) => {
      e.stopPropagation();
      const was = wrap.classList.contains("open");
      refreshMenuAvailability();
      closeMenus();
      if (!was) wrap.classList.add("open");
    };
    wrap.append(b, menuEntries(items));
    bar.append(wrap);
  }
  refreshMenuAvailability();
}
function closeMenus() {
  document
    .querySelectorAll(".menu.open")
    .forEach((x) => x.classList.remove("open"));
}
document.addEventListener("click", (e) => {
  if (!e.target.closest(".menu")) closeMenus();
});
function rememberSelection() {
  const s = getSelection();
  if (s?.rangeCount && $("#editor").contains(s.anchorNode))
    lastRange = s.getRangeAt(0).cloneRange();
}
function restoreSelection() {
  if (source) {
    $("#source").focus();
    return;
  }
  editor?.focus();
  if (lastRange && lastRange.startContainer.isConnected) {
    const s = getSelection();
    s.removeAllRanges();
    s.addRange(lastRange);
  }
}
function content() {
  return active?.content ?? "";
}
function normalizeMedia(value) {
  return restoreMediaReferences(value, active?.mediaUrls);
}
const punctuationView=new PunctuationView();
let punctuationFrame=0;
function schedulePunctuationView(){
  cancelAnimationFrame(punctuationFrame);
  punctuationFrame=requestAnimationFrame(()=>{
    if(currentComposition||!ready||loading)return;
    applyEditorSpellcheck();
    punctuationView.apply($('#editor .vditor-ir > .vditor-reset'),!source&&!!settings.markdown.smartRender);
    scheduleOfflineSpelling();
  });
}
function applyEditorSpellcheck(){
  const enabled=effectiveSpelling(active,settings).enabled,root=$('#editor .vditor-ir > .vditor-reset');
  $('#source').spellcheck=enabled;
  if(root)root.spellcheck=enabled;
  // Typora offers suggestions in inline code, but excludes fenced code.
  root?.querySelectorAll('code').forEach(node=>{
    node.spellcheck=enabled&&!node.closest('pre:not(.vditor-reset),[data-type="code-block"]');
  });
}
let offlineSpell=null,offlineSpellLanguage='',offlineSpellTimer;
const offlineDictionaries={},offlineDictionaryLoads={};
function spellingCorrect(word){return isPersonalWord(word,settings.personalWords)||offlineSpell.correct(word);}
function scheduleOfflineSpelling(){
  clearTimeout(offlineSpellTimer);
  const mode=effectiveSpelling(active,settings);
  if(!mode.enabled||source){CSS.highlights?.delete('markdownreader-spelling');return;}
  offlineSpellLanguage=mode.language;offlineSpell=offlineDictionaries[mode.language]||null;
  if(!offlineSpell){
    offlineDictionaryLoads[mode.language]??=(mode.language==='en-GB'?import('./british-dictionary.js'):import('./english-dictionary.js')).then(module=>{offlineDictionaries[mode.language]=module.default;scheduleOfflineSpelling();}).catch(error=>{delete offlineDictionaryLoads[mode.language];notify('无法加载离线英文词典：'+error.message,true);});
    return;
  }
  offlineSpellTimer=setTimeout(()=>{
    const current=effectiveSpelling(active,settings);
    if(currentComposition||!ready||loading||source||!current.enabled||current.language!==mode.language)return;
    const ranges=[];
    for(const node of spellingTextNodes($('#editor .vditor-ir > .vditor-reset'))){
      for(const word of englishWords(node.data))if(!spellingCorrect(word.word)){
        const range=document.createRange();range.setStart(node,word.start);range.setEnd(node,word.end);ranges.push(range);
      }
    }
    if(CSS.highlights&&globalThis.Highlight){const highlight=new Highlight();for(const range of ranges)highlight.add(range);CSS.highlights.set('markdownreader-spelling',highlight);}
  },180);
}
let spellingPopup=null;
function closeSpellingPopup(){spellingPopup?.remove();spellingPopup=null;}
document.addEventListener('pointerdown',event=>{if(!spellingPopup?.contains(event.target))closeSpellingPopup();},true);
document.addEventListener('keydown',event=>{if(event.key==='Escape')closeSpellingPopup();},true);
$('#editor').addEventListener('contextmenu',event=>{
  const mode=effectiveSpelling(active,settings);
  if(source||!mode.enabled||!offlineSpell||offlineSpellLanguage!==mode.language||currentComposition)return;
  const root=$('#editor .vditor-ir > .vditor-reset'),caret=document.caretRangeFromPoint(event.clientX,event.clientY);
  if(!caret||!root?.contains(caret.startContainer)||caret.startContainer.nodeType!==3)return;
  if(!spellingTextNodes(root).includes(caret.startContainer))return;
  const selection=getSelection();selection.removeAllRanges();selection.addRange(caret);punctuationView.restore();
  const node=selection.anchorNode,offset=selection.anchorOffset,word=englishWords(node.data).find(w=>offset>=w.start&&offset<=w.end);
  if(!word||spellingCorrect(word.word))return;
  const range=document.createRange();range.setStart(node,word.start);range.setEnd(node,word.end);selection.removeAllRanges();selection.addRange(range);rememberSelection();
  const [start,end]=currentSelection(),original=content(),owner=active;
  if(original.slice(start,end)!==word.word)return;
  event.preventDefault();event.stopImmediatePropagation();closeSpellingPopup();
  const languageName=mode.language==='en-GB'?'英式英语':'美式英语';
  const popup=document.createElement('div');spellingPopup=popup;popup.id='spelling-popup';popup.setAttribute('role','menu');popup.setAttribute('aria-label',languageName+'拼写建议');
  const label=document.createElement('small');label.textContent=languageName+' · '+word.word;popup.append(label);
  const suggestions=offlineSpell.suggest(word.word);
  for(const suggestion of suggestions){
    const button=document.createElement('button');button.type='button';button.setAttribute('role','menuitem');button.textContent=suggestion;
    button.onmousedown=e=>e.preventDefault();
    button.onclick=()=>{closeSpellingPopup();if(active!==owner||content()!==original)return notify('文档已变化，请重新选择错词',true);applyTextEdit(start,end,suggestion,true);schedulePunctuationView();};popup.append(button);
  }
  if(!suggestions.length){const empty=document.createElement('span');empty.textContent='没有建议';popup.append(empty);}
  if(word.word.length<=64){
    const add=document.createElement('button');add.type='button';add.setAttribute('role','menuitem');add.textContent='添加到字典';add.dataset.action='add-personal-word';
    add.onmousedown=e=>e.preventDefault();
    add.onclick=()=>{closeSpellingPopup();safely(async()=>{const next=await api.AddPersonalWord(word.word);acceptSettings(next);applyAppearance();});};popup.append(add);
  }
  document.body.append(popup);popup.style.left=Math.max(4,Math.min(event.clientX,innerWidth-popup.offsetWidth-4))+'px';popup.style.top=Math.max(4,Math.min(event.clientY,innerHeight-popup.offsetHeight-4))+'px';
},true);
function serializeEditor() {
  return punctuationView.originalText(()=>{
  const serialized=serializeReferenceImages($("#editor .vditor-ir > .vditor-reset"),editor.vditor.lute);
  return normalizeMedia(serialized ?? editor.getValue());
  });
}
function captureRenderedBaseline() {
  if (active && editor) renderedBaseline = {id:active.id,source:content(),serialized:serializeEditor()};
}
function readEditor() {
  const serialized=serializeEditor();
  const value=renderedBaseline?.id===active?.id && renderedBaseline.source===content()
    ? preserveRenderedSource(content(),renderedBaseline.serialized,serialized,diff) : serialized;
  renderedBaseline = {id:active.id,source:value,serialized};
  return value;
}
async function patchImages(root) {
  if (!active) return;
  const owner = active;
  // Media resolution depends on YAML in the current revision, including undo.
  await flush();
  if(active!==owner)return;
  restoreDataSVGImages(root, content());
  for (const img of root.querySelectorAll("img")) {
    let reference =
      img.getAttribute("data-media-original") || img.getAttribute("src") || "";
    if (reference.startsWith(location.origin))
      reference = reference.slice(location.origin.length);
    if (owner.baseURL && reference.startsWith(owner.baseURL))
      reference = reference.slice(owner.baseURL.length);
    if (!reference || /^(https?:|data:|blob:)/.test(reference)) continue;
    try {
      const path = await api.ResolveMedia(owner.id, reference);
      owner.mediaUrls ??= {};
      owner.mediaUrls[path] = reference;
      owner.mediaUrls[location.origin + path] = reference;
      if (path.startsWith(owner.baseURL) && owner.baseURL)
        owner.mediaUrls[path.slice(owner.baseURL.length)] = reference;
      img.setAttribute("data-media-original", reference);
      img.src = path;
    } catch {
      img.title = "图片无法读取：" + reference;
    }
  }
}
function changed(value, recordHistory = true) {
  if (loading || !active) return;
  value = normalizeMedia(value);
  if (renderedBaseline?.id !== active.id || renderedBaseline.source !== value) renderedBaseline = null;
  if (value === active.content && recordHistory) return;
  const before = active.content;
  active.content = value;
  if (recordHistory) {
    const pending = pendingInput; pendingInput = null;
    rememberSelection();
    const transaction = pending?.documentId === active.id && performance.now() - pending.time < 2000 ? {...pending, after: currentSelection()} : null;
    addHistory(active, before, value, transaction);
  }
  active.dirty = !!active.detached || value !== active.savedContent || active.eol !== active.savedEOL || active.encoding !== active.savedEncoding;
  active.revision = ++revision;
  clearTimeout(syncTimer);
  syncTimer = setTimeout(() => safely(flush), 350);
  updateStatus();
  renderTabs();
  renderOutline();
  if (settings.autoSave && active.path) {
    clearTimeout(saveTimer);
    saveTimer = setTimeout(() => safely(() => save(false, true)), 1600);
  }
  scheduleSearchHighlights();
}
async function flush() {
  clearTimeout(syncTimer);
  if (!active) return;
  const d = { ...active };
  syncChain = syncChain
    .catch(() => {})
    .then(() => api.UpdateDocument(d.id, d.content, d.eol, d.revision || 0));
  await syncChain;
}
async function configureParserMode() {
  if (!window.Lute) await Vditor.md2html('', {cdn});
  if (typeof Lute.SetMarkdownReaderDefaultStrictMode !== 'function') throw Error('解析器资源版本不匹配，请重新解压完整程序');
  Lute.SetMarkdownReaderDefaultStrictMode(settings.markdown.strictMode !== false);
  editor?.vditor?.lute?.SetMarkdownReaderStrictMode(settings.markdown.strictMode !== false);
}
async function configureMathEngine() {
  await prepareMathJax(cdn, !!settings.markdown.physics, settings.markdown.mathLineBreaks !== false);
  if (editor?.vditor) editor.vditor.options.preview.math.engine = "MarkdownReader";
}
function mathOptions() {return {cdn,physics:!!settings.markdown.physics,lineBreaks:settings.markdown.mathLineBreaks!==false,numbering:!!settings.markdown.mathNumbering};}
async function renderEditorMath(force=false) {
  const root=$("#editor .vditor-ir > .vditor-reset"),owner=active,options=mathOptions(),key=JSON.stringify(options);
  if(!root||source)return;
  await renderDocumentMath(root,options,{editor:true,force,isCurrent:()=>active===owner&&!source&&root===$("#editor .vditor-ir > .vditor-reset")&&JSON.stringify(mathOptions())===key});
}
function watchEditorMath() {
  mathObserver?.disconnect();clearTimeout(mathRenderTimer);
  const root=$("#editor .vditor-ir > .vditor-reset");if(!root)return;
  mathObserver=new MutationObserver(()=>{
    clearTimeout(mathRenderTimer);
    mathRenderTimer=setTimeout(()=>{if(!currentComposition&&!source)safely(async()=>{await renderEditorMath();schedulePunctuationView();});},80);
  });
  mathObserver.observe(root,{childList:true,subtree:true,characterData:true});
}
async function mountEditor() {
  punctuationView.restore();
  mathObserver?.disconnect();clearTimeout(mathRenderTimer);
  renderedBaseline = null;
  ready = false;
  loading = true;
  const initial = content();
  await configureParserMode();
  await configureMathEngine();
  if (editor) editor.destroy();
  $("#editor").innerHTML = "";
  await new Promise((resolve, reject) => {
    let timeout = setTimeout(
      () => reject(Error("编辑器加载超时，请检查本地资源")),
      20000,
    );
    editor = new Vditor("editor", {
      cdn,
      lang: "zh_CN",
      mode: "ir",
      undoDelay: 0,
      link: {isOpen: false},
      height: "100%",
      width: "100%",
      value: initial,
      cache: { enable: false },
      toolbar: [
        "headings",
        "bold",
        "italic",
        "strike",
        "link",
        "list",
        "ordered-list",
        "check",
        "outdent",
        "indent",
        "quote",
        "line",
        "code",
        "inline-code",
        "table",
        "undo",
        "redo",
      ],
      toolbarConfig: { hide: true },
      resize: { enable: false },
      icon: "ant",
      placeholder: "开始写作…",
      counter: { enable: false },
      hint: { emojiPath: cdn + "/dist/images/emoji" },
      tab: " ".repeat(settings.indent || 2),
      preview: {
        parse: (element) => safely(() => patchImages(element)),
        delay: 150,
        maxWidth: 860,
        math: { engine: "MarkdownReader", inlineDigit: !!settings.markdown.inlineMathDigit },
        hljs: {
          lineNumber: settings.lineNumbers,
          style: settings.theme === "Night" ? "dracula" : "github",
        },
        markdown: {
          sanitize: true,
          toc: true,
          mark: settings.markdown.mark,
          sub: settings.markdown.sub,
          sup: settings.markdown.sup,
          footnotes: true,
          paragraphBeginningSpace: !!settings.paragraphIndent,
          linkBase: active?.baseURL || "",
          codeBlockPreview: true,
          mathBlockPreview: true,
        },
        theme: { current: "light", path: cdn + "/dist/css/content-theme" },
        actions: [],
      },
      input: () => changed(readEditor()),
      after: () => {
        clearTimeout(timeout);
        ready = true;
        loading = false;
        applyAppearance();
        editor.focus();
        resolve();
      },
    });
  });
  $("#source").value = initial;
  $("#source").hidden = !source;
  $("#editor").hidden = source;
  applyAppearance();
  await patchImages($("#editor"));
  await renderEditorMath();
  watchEditorMath();
  captureRenderedBaseline();
  schedulePunctuationView();
  renderOutline();
  scheduleSearchHighlights();
}
async function switchDoc(d) {
  if (active?.id === d.id) return;
  if (active) {
    await flush();
    if (settings.saveOnSwitch && active.dirty) {
      if (!(await save())) return;
    }
  }
  active = d;
  if(!('savedContent' in active)) { active.savedContent=active.dirty?null:active.content; active.savedEOL=active.eol; active.savedEncoding=active.encoding }
  revision = d.revision || 0;
  lastRange = null;
  findPos = -1;
  $("#conflict").hidden = true;
  await mountEditor();
  renderTabs();
  updateStatus();
}
async function addDocument(d) {
  if (!d) return;
  const old = docs.find((x) => x.id === d.id);
  if (!old) docs.push(d);
  await switchDoc(old || d);
}
function renderTabs() {
  $("#tabs").innerHTML = "";
  $("#tabs").hidden = docs.length < 2;
  for (const d of docs) {
    const tab = document.createElement("div");
    tab.className = "document-tab" + (d.id === active?.id ? " active" : "");
    const select = document.createElement("button");
    select.className = "tab-select";
    select.textContent = (d.dirty ? "● " : "") + d.name;
    select.title = d.path || "未保存文档";
    select.onclick = () => safely(() => switchDoc(d));
    const close = document.createElement("button");
    close.className = "tab-close";
    close.textContent = "×";
    close.title = "关闭文档 (Ctrl+W)";
    close.setAttribute("aria-label", "关闭 " + d.name);
    close.onclick = () => safely(async () => {
      if (active?.id !== d.id) await switchDoc(d);
      await closeDocument();
    });
    tab.append(select, close);
    $("#tabs").append(tab);
  }
  const recent = menus[0][1].find((x) => x?.id === "recent");
  recent.children = (settings.recent || []).map((p) => cmd(p, "openpath:" + p));
  buildMenus();
}
function updateStatus() {
  if (!active) return;
  updateDocumentStatistics();
  $("#file-state").textContent =
    (active.dirty ? "未保存 · " : "") + active.eol + " · " + active.encoding;
  $("#mode").textContent = source ? "源代码" : "即时渲染";
  api.SetTitle((active.dirty ? "* " : "") + active.name).catch(() => {});
}
function updateDocumentStatistics() {
  if (!active || !settings) return;
  if (statisticsSelection?.id !== active.id) statisticsSelection = null;
  if (source && document.activeElement === $("#source")) {
    const field = $("#source");
    statisticsSelection = field.selectionStart === field.selectionEnd ? null :
      {id:active.id, text:field.value.slice(field.selectionStart,field.selectionEnd)};
  } else if (!source) {
    const sel = getSelection(), root = $("#editor .vditor-ir .vditor-reset");
    if (root && sel?.rangeCount && root.contains(sel.anchorNode) && root.contains(sel.focusNode)) {
      statisticsSelection = sel.isCollapsed ? null : {id:active.id, text:sel.toString()};
    }
  }
  const total = documentStatistics(content(), settings.readingSpeed, settings.markdown.strictMode).words;
  const selected = statisticsSelection && documentStatistics(statisticsSelection.text, settings.readingSpeed, settings.markdown.strictMode).words;
  $("#count").textContent = statisticsSelection && selected !== total ? `${selected} / ${total} 词` : `${total} 词`;
}
async function save(saveAs = false, quiet = false) {
  if (!active) return false;
  const owner = active;
  const previousBaseURL = owner.baseURL;
  await flush();
  const d = await api.SaveDocument(owner.id, saveAs);
  if (!d) return false;
  Object.assign(owner, d);
  owner.savedContent=owner.content;owner.savedEOL=owner.eol;owner.savedEncoding=owner.encoding;
  if (previousBaseURL !== owner.baseURL) {
    owner.mediaUrls = {};
    if (active === owner) {
      lastRange = null;
      await mountEditor();
    }
  }
  acceptSettings(await api.GetSettings());
  renderTabs();
  updateStatus();
  if (!quiet) notify("已保存：" + d.path);
  return true;
}
async function closeDocument() {
  if (!active) return;
  await flush();
  if (active.dirty) {
    const r = await dialog(
      "未保存的更改",
      `保存 ${esc(active.name)} 后关闭？`,
      [
        ["保存", "save"],
        ["不保存", "discard"],
        ["取消", "cancel"],
      ],
    );
    if (r.value === "cancel") return;
    if (r.value === "save" && !(await save())) return;
  }
  const id = active.id;
  await api.CloseDocument(id);
  docs = docs.filter((d) => d.id !== id);
  active = null;
  if (docs.length) await switchDoc(docs[0]);
  else await addDocument(await api.NewDocument());
}
async function toggleSource() {
  punctuationView.restore();
  let range = null;
  if (!source) range = markdownSelection();
  else
    active.sourceCursor = [
      $("#source").selectionStart,
      $("#source").selectionEnd,
    ];
  await flush();
  source = !source;
  schedulePunctuationView();
  $("#findbar").hidden = true;
  clearSearchHighlights();
  refreshMenuAvailability();
  $("#source").value = content();
  $("#source").hidden = !source;
  $("#editor").hidden = source;
  if (source) {
    $("#source").focus();
    if (range) $("#source").setSelectionRange(range[0], range[1]);
  } else {
    await configureParserMode();
    await configureMathEngine();
    editor.setValue(content());
    await patchImages($("#editor"));
    await renderEditorMath();
    watchEditorMath();
    captureRenderedBaseline();
    setMarkdownSelection(...(active.sourceCursor || [0, 0]));
  }
  updateStatus();
  scheduleSearchHighlights();
}
function markdownSelection() {
  const root = $("#editor .vditor-ir .vditor-reset");
  const live = getSelection()?.rangeCount ? getSelection().getRangeAt(0) : null;
  const range = live && root?.contains(live.startContainer) && root.contains(live.endContainer)
    ? live : lastRange;
  if (!root || !range || !root.contains(range.startContainer))
    return active.sourceCursor || [0, 0];
  try {
    const canonical = editableText(root).text;
    const patches = diff.diff_main(canonical, content());
    const position = (node, offset) => {
      const prefix = document.createRange();
      prefix.selectNodeContents(root);
      prefix.setEnd(node, offset);
      const div = document.createElement("div");
      div.append(prefix.cloneContents());
      return diff.diff_xIndex(patches, editableText(div).text.length);
    };
    return [
      position(range.startContainer, range.startOffset),
      position(range.endContainer, range.endOffset),
    ];
  } catch {
    return active.sourceCursor || [0, 0];
  }
}
function editableText(root) {
  const nodes = [];
  let text = "";
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  while (walker.nextNode()) {
    const node = walker.currentNode;
    if (node.parentElement?.closest('.vditor-ir__preview,[contenteditable="false"],wbr')) continue;
    nodes.push({node, start: text.length});
    text += node.data;
  }
  return {text, nodes};
}
function setMarkdownSelection(start, end = start, showMarkers = false) {
  active.sourceCursor = [start, end];
  active.historySelections ??= [];
  active.historySelections[active.historyIndex || 0] = [start, end];
  if (source) {
    const t = $("#source");
    t.focus();
    t.setSelectionRange(start, end);
    const line = content().slice(0, start).split("\n").length;
    t.scrollTop = Math.max(0, (line - 1) * settings.fontSize * 1.7 - t.clientHeight / 2);
    return;
  }
  const root = $("#editor .vditor-ir .vditor-reset");
  const {text, nodes} = editableText(root);
  const patches = diff.diff_main(content(), text);
  const point = (offset) => {
    offset = diff.diff_xIndex(patches, offset);
    for (const entry of nodes) {
      if (offset <= entry.start + entry.node.length)
        return [entry.node, Math.max(0, offset - entry.start)];
    }
    return [root, root.childNodes.length];
  };
  const range = document.createRange();
  range.setStart(...point(start));
  range.setEnd(...point(end));
  root.focus();
  getSelection().removeAllRanges();
  getSelection().addRange(range);
  if (showMarkers) {
    for (const node of root.querySelectorAll('.vditor-ir__node')) {
      if (range.intersectsNode(node)) node.classList.add('vditor-ir__node--expand');
    }
  }
  lastRange = range.cloneRange();
  const element = range.startContainer.parentElement;
  element?.scrollIntoView({block: "nearest"});
}
function insert(value) {
  restoreSelection();
  if (source) {
    const t = $("#source"),
      start = t.selectionStart,
      end = t.selectionEnd;
    t.setRangeText(value, start, end, "end");
    changed(t.value);
  } else {
    editor.insertValue(value, true);
    changed(readEditor());
  }
  lastRange = null;
}
function selection() {
  return source
    ? $("#source").value.slice(
        $("#source").selectionStart,
        $("#source").selectionEnd,
      )
    : editor.getSelection();
}
function wrap(prefix, suffix = prefix) {
  const selected = selection();
  insert(prefix + selected + suffix);
}
function toolbar(type) {
  restoreSelection();
  const b = $(`#editor .vditor-toolbar [data-type="${type}"]`);
  if (b) {
    b.click();
    changed(readEditor());
    lastRange = null;
    return true;
  }
  return false;
}
async function editSource(fn) {
  const [a, b] = currentSelection();
  const r = fn(content(), a, b);
  if (!r) return;
  applyTextEdit(r.start, r.end, r.text);
}
function currentSelection() {
  return source ? [$("#source").selectionStart, $("#source").selectionEnd] : markdownSelection();
}
function applyTextEdit(start, end, text, collapse = false) {
  rememberHistoryCursor();
  const value = content().slice(0, start) + text + content().slice(end);
  changed(value);
  if (source) $("#source").value = value;
  else { editor.setValue(value); captureRenderedBaseline(); safely(() => patchImages($("#editor"))); }
  setMarkdownSelection(collapse ? start + text.length : start, start + text.length);
}
function rememberHistoryCursor() {
  if (!active) return;
  active.historySelections ??= [];
  active.historySelections[active.historyIndex || 0] = currentSelection();
}
async function selectRange(kind, remove = false) {
  const v = content();
  let [a, b] = currentSelection();
  let showMarkers = kind === 'format';
  if (kind === "all") {
    a = 0;
    b = v.length;
  } else if (kind === "line" || kind === "block") {
    [a, b] = lineBounds(v, a, b);
    if (kind === "line" && !source && !remove) {
      const selectedNode = getSelection()?.anchorNode;
      const element = selectedNode?.nodeType === Node.ELEMENT_NODE ? selectedNode : selectedNode?.parentElement;
      if (!element?.closest('pre:not(.vditor-reset),[data-type="code-block"],.vditor-ir__math'))
        [a, b] = sentenceBounds(v, ...currentSelection());
      else showMarkers = true;
    }
    if (kind === "block") {
      while (
        a > 0 &&
        v
          .slice(0, a - 1)
          .split("\n")
          .at(-1)
          ?.trim()
      ) {
        a = v.lastIndexOf("\n", a - 2) + 1;
      }
      while (
        b < v.length &&
        v
          .slice(b + 1)
          .split("\n")[0]
          ?.trim()
      ) {
        const n = v.indexOf("\n", b + 1);
        b = n < 0 ? v.length : n;
      }
    }
  } else if (kind === "word" || kind === "format") {
    const format=kind==='format'?formatRange(v,a,b):null;
    if(format)[a,b]=format;
    else {
      while (a > 0 && /[\p{L}\p{N}_]/u.test(v[a - 1])) a--;
      while (b < v.length && /[\p{L}\p{N}_]/u.test(v[b])) b++;
    }
  }
  if (remove) applyTextEdit(a, b, "", true);
  else setMarkdownSelection(a, b, showMarkers);
}
function tableContext() {
  if (!active) return null;
  if (source) return tableAt(content(), $("#source").selectionStart);
  const range = lastRange?.startContainer.isConnected ? lastRange : (getSelection()?.rangeCount ? getSelection().getRangeAt(0) : null);
  const node = range?.startContainer;
  const cell = (node?.nodeType === 1 ? node : node?.parentElement)?.closest("td,th");
  if (!cell || !$("#editor").contains(cell)) return null;
  const tableElement = cell.closest("table");
  const tableIndex = [...$("#editor .vditor-ir .vditor-reset").querySelectorAll("table")].indexOf(tableElement);
  const table = markdownTables(content())[tableIndex];
  if (!table) return null;
  return {...table, row: cell.parentElement.rowIndex, column: cell.cellIndex};
}
async function tableCommand(action) {
  const context = tableContext();
  if (!context) return notify("请先将光标放入表格单元格", true);
  if (action === "copy") {
    await writeClipboard(content().slice(context.start, context.end));
    return notify("已复制表格 Markdown");
  }
  let options = {};
  if (action === "resize") {
    const result = await dialog("调整表格", `<label>列数<input name="cols" type="number" min="1" max="100" value="${context.align.length}"></label><label>行数（含表头）<input name="rows" type="number" min="1" max="100" value="${context.rows.length}"></label><p>缩小表格会移除范围外的单元格，可用撤销恢复。</p>`, [["调整", "ok"],["取消", "cancel"]]);
    if (result.value !== "ok") return;
    options = result.data;
  }
  const edit = transformTable(context, action, options);
  if (!edit) return;
  applyTextEdit(edit.start, edit.end, edit.text, true);
  active.historySelections ??= [];
  active.historySelections[active.historyIndex || 0] = [edit.cursor, edit.cursor];
  if (source) setMarkdownSelection(edit.cursor);
  else if (edit.text) {
    const table = $("#editor .vditor-ir .vditor-reset").querySelectorAll("table")[[...markdownTables(content())].findIndex(t=>t.start===edit.start)];
    const cell = table?.rows[edit.row]?.cells[edit.column];
    if (cell) {
      const range = document.createRange(); range.selectNodeContents(cell); range.collapse(true);
      cell.closest('[contenteditable="true"]')?.focus();
      getSelection().removeAllRanges(); getSelection().addRange(range); lastRange = range.cloneRange();
      cell.scrollIntoView({block:"nearest"});
    }
  }
  updateTableTools();
}
const tableTools = document.createElement("div");
tableTools.id = "table-tools";
tableTools.hidden = true;
tableTools.setAttribute("aria-label", "表格编辑工具");
for (const [label, action] of [["调整表格", "resize"],["左对齐", "align-left"],["居中", "align-center"],["右对齐", "align-right"],["＋行", "row-below"],["＋列", "column-right"]]) {
  const button = document.createElement("button"); button.textContent = label;
  button.onmousedown = e => e.preventDefault();
  button.onclick = () => safely(() => tableCommand(action));
  tableTools.append(button);
}
$("#work").append(tableTools);
function updateTableTools() {
  tableTools.hidden = !tableContext();
}
function currentLink(image) {
  const position=currentSelection()[0];
  const found = image?documentImages(content()).find(l=>position>=l.start&&position<=l.end):linkAt(content(),position,false);
  if (found) return found;
  if (lastLinkContext?.id===active.id && lastLinkContext.content===content() && lastLinkContext.link.image===image) return lastLinkContext.link;
  return null;
}
function jumpToAnchor(fragment) {
  if (!fragment) return;
  let value=fragment;try {value=decodeURIComponent(fragment);}catch{}
  const slug=s=>s.toLowerCase().replace(/[\s]+/g,"-").replace(/[^\p{L}\p{N}_-]/gu,"");
  if (source) {
    const match=[...content().matchAll(/^#{1,6}\s+(.+)$/gm)].find(m=>slug(m[1])===slug(value));
    if (!match) throw Error("未找到文档标题："+value);
    return setMarkdownSelection(match.index);
  }
  const heading=[...$("#editor").querySelectorAll("h1,h2,h3,h4,h5,h6")].find(h=>h.id===value||h.id.replace(/^ir-/,"").replace(/_\d+$/,"")===value||slug(h.textContent.replace(/^#+\s*/,""))===slug(value));
  if (!heading) throw Error("未找到文档标题："+value);
  heading.scrollIntoView({block:"start"});
}
async function openDocumentLink(destination) {
  await flush();
  const target=await api.ResolveLink(active.id,destination);
  if(target.kind==="anchor") return jumpToAnchor(target.fragment);
  if(target.kind==="markdown") {
    await addDocumentFromPath(target.path);
    return jumpToAnchor(target.fragment);
  }
  await api.OpenLink(active.id,destination);
}
async function editLink(image, requireExisting=false) {
  const link=currentLink(image);
  if(requireExisting&&!link) return notify(image?"请先选中图片":"请先将光标放入链接",true);
  const owner=active, original=content();
  const [start,end]=link?[link.start,link.end]:currentSelection();
  const result=await dialog(image?"图片地址":"超链接", `<label>${image?"替代文字":"显示文字"}<input name="label" value="${esc(link?.label||selection()|| (image?"图像":"链接文字"))}"></label><label>地址<input name="destination" value="${esc(link?.destination||"https://")}" autocomplete="off"></label><label>标题（可选）<input name="title" value="${esc(link?.title||"")}"></label>`, [["确定","ok"],["取消","cancel"]]);
  if(result.value!=="ok")return;
  if(!result.data.destination.trim())throw Error("地址不能为空");
  if(active!==owner||content()!==original)throw Error("文档已变化，请重新插入链接或图片");
  let destination=result.data.destination;
  if(image&&!link){setMarkdownSelection(start,end);await flush();destination=await api.PrepareImageReference(owner.id,destination,result.data.label);if(active!==owner||content()!==original||currentSelection().some((offset,i)=>offset!==[start,end][i]))throw Error("文档或光标已变化，未插入处理后的图片地址");}
  applyTextEdit(start,end,writeLink(result.data.label,destination,result.data.title,image));
  lastLinkContext=null;
}
async function linkCommand(action,image) {
  if(action==="edit")return editLink(image,true);
  const link=currentLink(image);
  if(!link)return notify(image?"请先选中图片":"请先将光标放入链接",true);
  if(action==='bitmap') {
    const owner=active;
    const png=(async()=>{
      const url=await api.ResolveMedia(owner.id,link.destination);
      const response=await fetch(url);
      if(!response.ok)throw Error('无法读取图片');
      const blob=await response.blob();
      if(blob.size>20*1024*1024)throw Error('图片不能超过20MB');
      let bitmap, objectURL;
      try { bitmap=await createImageBitmap(blob); }
      catch {
        // Chromium's createImageBitmap does not decode every SVG it can show.
        objectURL=URL.createObjectURL(blob);
        const picture=new Image();picture.src=objectURL;
        try { await picture.decode(); }
        catch(error) { URL.revokeObjectURL(objectURL);throw error; }
        bitmap={width:picture.naturalWidth,height:picture.naturalHeight,picture,close:()=>URL.revokeObjectURL(objectURL)};
      }
      try {
        if(bitmap.width<=0||bitmap.height<=0||bitmap.width*bitmap.height>16*1024*1024)throw Error('图片尺寸无效或超过1600万像素');
        const canvas=document.createElement('canvas');canvas.width=bitmap.width;canvas.height=bitmap.height;
        canvas.getContext('2d').drawImage(bitmap.picture||bitmap,0,0);
        return await new Promise((resolve,reject)=>canvas.toBlob(blob=>blob?resolve(blob):reject(Error('无法转换图片为剪贴板位图')),'image/png'));
      } finally { bitmap.close(); }
    })();
    // Supply the promise while the menu click still has a user activation.
    await navigator.clipboard.write([new ClipboardItem({'image/png':png})]);
    return notify('已拷贝图片');
  }
  if(action==="copy"){await writeClipboard(link.destination);return notify("已复制地址");}
  if(action==="open")return openDocumentLink(link.destination);
  if(action==="reveal"){
    if(image)return api.Reveal(await api.ImageLocation(active.id,link.destination));
    const target=await api.ResolveLink(active.id,link.destination);
    if(!target.path)throw Error("网络图片没有本地文件位置");
    return api.Reveal(target.path);
  }
  if(action==="remove")return applyTextEdit(link.start,link.end,link.label);
}
$("#editor").addEventListener("click", e=>{
  const mathLink=e.target.closest('mjx-container a');
  if(mathLink){
    const href=mathLink.getAttribute('href')||mathLink.getAttributeNS('http://www.w3.org/1999/xlink','href');
    if(href?.startsWith('#')){e.preventDefault();try{const target=document.getElementById(decodeURIComponent(href.slice(1)));if(target&&$("#editor").contains(target))target.scrollIntoView({block:'center'});}catch{}return;}
  }
  const element=e.target.closest('[data-type="a"],a,img');
  if(!element)return;
  e.preventDefault();
  const image=element.tagName==="IMG";
  const destination=image?(element.getAttribute("data-media-original")||element.getAttribute("src")):(element.querySelector(".vditor-ir__marker--link")?.textContent||element.getAttribute("href"));
  const candidates=(image?documentImages(content()):markdownLinks(content())).filter(l=>l.image===image&&l.destination===destination);
  const ordinal=image?[...$("#editor").querySelectorAll('img')].filter(img=>(img.getAttribute('data-media-original')||img.getAttribute('src'))===destination).indexOf(element):0;
  const link=candidates[Math.max(0,ordinal)];
  if(link){
    lastLinkContext={id:active.id,content:content(),link};
    if(image)setMarkdownSelection(link.start,link.end);
  }
  if(e.ctrlKey&&link&&!image){e.stopImmediatePropagation();safely(()=>openDocumentLink(link.destination));}
},true);
async function navigateTable(backward) {
  const context = tableContext();
  if (!context || source) return false;
  rememberHistoryCursor();
  let row = context.row, column = context.column + (backward ? -1 : 1);
  if (column < 0) { row--; column = context.align.length - 1; }
  if (column >= context.align.length) { row++; column = 0; }
  if (row >= context.rows.length || row < 0) {
    // Typora adds below the last row, or below the header at the reverse edge.
    const edge = row < 0;
    const edit = transformTable({...context, row: edge ? 0 : context.rows.length - 1, column: 0}, "row-below");
    applyTextEdit(edit.start, edit.end, edit.text, true);
    row = edit.row; column = 0;
  }
  const index = markdownTables(content()).findIndex(t => t.start === context.start);
  const cell = $("#editor .vditor-ir .vditor-reset").querySelectorAll("table")[index]?.rows[row]?.cells[column];
  if (cell) {
    const range = document.createRange(); range.selectNodeContents(cell);
    cell.closest('[contenteditable="true"]')?.focus();
    getSelection().removeAllRanges(); getSelection().addRange(range);
    lastRange = range.cloneRange();
    rememberHistoryCursor();
    cell.scrollIntoView({block:"nearest"});
  }
  updateTableTools();
  return true;
}
function applyAppearance() {
  const theme = settings.theme || "Github";
  document.body.dataset.theme = theme;
  document.body.classList.toggle("focus-mode", !!settings.focus);
  document.body.classList.toggle("typewriter", !!settings.typewriter);
  document.body.classList.toggle("code-wrap", !!settings.codeWrap);
  document.body.classList.toggle("collapse-soft-breaks", settings.markdown.keepSingleLineBreaks === false);
  document.body.classList.toggle("show-br-markers", settings.markdown.showBRMarkers !== false);
  decorateBreakMarkers($("#editor"));
  document.documentElement.style.setProperty(
    "--font-size",
    settings.fontSize * zoom + "px",
  );
  $("#source").style.tabSize = settings.indent;
  $("#source").spellcheck = !!settings.spellcheck;
  document
    .querySelectorAll("[contenteditable]")
    .forEach((el) => (el.spellcheck = !!settings.spellcheck));
  applyEditorSpellcheck();
  scheduleOfflineSpelling();
  if (editor && ready)
    editor.setTheme(
      theme === "Night" ? "dark" : "classic",
      theme === "Night" ? "dark" : "light",
      theme === "Night" ? "dracula" : "github",
      cdn + "/dist/css/content-theme",
    );
  $("#user-css")?.remove();
  if (settings.customCSS) {
    const st = document.createElement("style");
    st.id = "user-css";
    st.textContent = settings.customCSS;
    document.head.append(st);
  }
  scheduleSearchHighlights();
}
async function persist() {
  settingsSaveBusy = true;
  try {
    await api.SetSettingsFrom(settings, settingsBaseline);
    acceptSettings(await api.GetSettings());
    applyAppearance();
  } finally { settingsSaveBusy = false; }
}
async function refreshWindowSettings() {
  if (!ready || loading || settingsSyncBusy || settingsSaveBusy || currentComposition || pendingModal || exportInProgress || performance.now() - lastInputAt < 1500) return;
  if (JSON.stringify(settings) !== JSON.stringify(settingsBaseline)) return;
  settingsSyncBusy = true;
  const baseline = settingsBaseline;
  try {
    await api.RefreshSettings();
    const next = await api.GetSettings();
    if (JSON.stringify(next) === JSON.stringify(settings) || baseline !== settingsBaseline || loading || currentComposition || pendingModal || settingsSaveBusy || performance.now() - lastInputAt < 1500) return;
    const rebuild = settings.indent !== next.indent || settings.lineNumbers !== next.lineNumbers || settings.paragraphIndent !== next.paragraphIndent || JSON.stringify(settings.markdown) !== JSON.stringify(next.markdown);
    const selection = currentSelection();
    const focused = document.activeElement;
    const workFocused = !!focused?.closest('#work');
    acceptSettings(next);
    buildMenus();
    applyAppearance();
    if (rebuild && active) {
      lastRange = null;
      await mountEditor();
      if (workFocused) setMarkdownSelection(...selection);
      else if (focused?.isConnected) focused.focus();
    }
  } finally { settingsSyncBusy = false; }
}
function showSidebar(kind = side) {
  side = kind;
  showSide = true;
  $("#sidebar").hidden = false;
  renderSide();
}
function renderSide() {
  document
    .querySelectorAll("[data-side]")
    .forEach((b) => b.classList.toggle("active", b.dataset.side === side));
  if (side === "outline") renderOutline();
  if (side === "list") renderList();
  if (side === "tree") renderTree();
  if (side === "search") renderFolderSearch();
}
function renderOutline() {
  if (side !== "outline") return;
  const container = $("#side-content");
  container.innerHTML = "";
  const headings = [];
  let fence = false;
  content()
    .split("\n")
    .forEach((l, i) => {
      if (/^\s*(```|~~~)/.test(l)) fence = !fence;
      if (fence) return;
      const headingPattern = settings.markdown.strictMode === false ? /^(#{1,6})(?!#)[ \t]*(.+?)(?:\s+#+)?$/ : /^(#{1,6})\s+(.+?)(?:\s+#+)?$/;
      const m = headingPattern.exec(l);
      if (m) headings.push({ level: m[1].length, text: m[2], line: i });
    });
  if (!headings.length) {
    container.innerHTML = '<p class="empty">添加标题后在这里显示大纲</p>';
    return;
  }
  for (const h of headings) {
    const b = document.createElement("button");
    b.className = "outline-item";
    b.style.paddingLeft = 12 + (h.level - 1) * 13 + "px";
    b.textContent = h.text;
    b.onclick = () => {
      if (source) {
        let start =
          content().split("\n").slice(0, h.line).join("\n").length +
          (h.line ? 1 : 0);
        $("#source").focus();
        $("#source").setSelectionRange(start, start);
        $("#source").scrollTop = h.line * settings.fontSize * 1.7;
      } else {
        const items = Array.from(
          document.querySelectorAll(
            "#editor .vditor-ir h1,#editor .vditor-ir h2,#editor .vditor-ir h3,#editor .vditor-ir h4,#editor .vditor-ir h5,#editor .vditor-ir h6",
          ),
        );
        const target = items[headings.indexOf(h)];
        target?.scrollIntoView({ block: "start", behavior: "smooth" });
      }
    };
    container.append(b);
  }
}
function renderList() {
  const container = $("#side-content");
  container.innerHTML = "";
  for (const d of docs) {
    const b = document.createElement("button");
    b.className = "file-item";
    b.textContent = (d.dirty ? "● " : "") + d.name;
    b.title = d.path;
    b.onclick = () => safely(() => switchDoc(d));
    container.append(b);
  }
  const b = document.createElement("button");
  b.textContent = "打开文件…";
  b.onclick = () => safely(() => run("open"));
  container.append(b);
}
function renderTree() {
  const c = $("#side-content");
  c.innerHTML = "";
  if (!folder) {
    c.innerHTML =
      '<p class="empty">打开文件夹以浏览文档</p><button id="choose-folder">打开文件夹…</button>';
    $("#choose-folder").onclick = () => safely(() => run("folder"));
    return;
  }
  const title = document.createElement("p");
  title.className = "folder-title";
  title.textContent = folder;
  title.title = folder;
  c.append(title);
  function nodes(items, parent) {
    for (const item of items) {
      if (item.dir) {
        const d = document.createElement("details");
        const s = document.createElement("summary");
        s.textContent = item.name;
        d.append(s);
        nodes(item.children || [], d);
        parent.append(d);
      } else {
        const b = document.createElement("button");
        b.className = "file-item";
        b.textContent = item.name;
        b.title = item.path;
        b.onclick = () => safely(() => addDocumentFromPath(item.path));
        b.oncontextmenu = e => {e.preventDefault();safely(()=>fileTreeActions(item.path,item.name));};
        parent.append(b);
      }
    }
  }
  nodes(tree, c);
}
async function addDocumentFromPath(p) {
  await flush();
  await addDocument(await api.OpenPath(p));
}
async function fileTreeActions(path,name) {
  const result=await dialog(name,"选择文件操作",[["打开","open"],["新窗口打开","window"],["重命名","rename"],["删除…","delete"],["取消","cancel"]]);
  if(result.value==="cancel")return;
  if(result.value==="window")return api.NewWindow(path);
  await addDocumentFromPath(path);
  if(result.value==="rename")await run("renamefile");
  if(result.value==="delete")await run("deletefile");
}
async function reopenEncoding(name) {
  if(!active.path)return notify("请先打开一个已保存的文件，或选择“按编码打开”",true);
  let discard=false;
  if(active.dirty){
    const result=await dialog("重新打开文档","这会放弃当前未保存的修改，重新读取磁盘文件。",[["重新打开","reopen"],["取消","cancel"]]);
    if(result.value!=="reopen")return;
    discard=true;
  }
  await flush();Object.assign(active,await api.ReopenEncoding(active.id,name,discard));
  active.savedContent=active.content;active.savedEOL=active.eol;active.savedEncoding=active.encoding;
  active.history=[active.content];active.historyIndex=0;active.historySelections=[[0,0]];active.historyEffects=[null];active.historyGroup=null;active.sourceCursor=[0,0];
  revision=active.revision;lastRange=null;await mountEditor();renderTabs();updateStatus();
}
async function openWithEncoding() {
  const result=await dialog("按编码打开",`<label>文本编码<select name="encoding"><option value="auto">自动</option>${encodingOptions.map(option=>`<option value="${esc(option.name)}">${esc(option.label)}</option>`).join("")}</select></label>`,[["选择文件…","open"],["取消","cancel"]]);
  if(result.value!=="open")return;
  await flush();await addDocument(await api.OpenDialogEncoding(result.data.encoding));
}
async function renameCurrentFile() {
  if(!active.path)return notify("请先保存文档",true);
  const name=await promptText("重命名文件","文件名",active.name);
  if(name===null)return;
  await flush();Object.assign(active,await api.RenameDocument(active.id,name));
  acceptSettings(await api.GetSettings());if(folder)tree=await api.FolderTree(folder);
  renderTabs();renderSide();updateStatus();notify("已重命名为 "+active.name);
}
async function deleteCurrentFile() {
  if(!active.path)return notify("此文档尚未保存为文件",true);
  const result=await dialog("删除文件",`<p>将 ${esc(active.name)} 移到系统回收站？</p><p>当前编辑内容会保留，可另存为新文件。也可从“恢复删除的文件”取回原文件。</p>`,[["删除文件","delete"],["取消","cancel"]]);
  if(result.value!=="delete")return;
  await flush();Object.assign(active,await api.DeleteDocumentFile(active.id));active.savedContent=null;
  acceptSettings(await api.GetSettings());if(folder)tree=await api.FolderTree(folder);
  renderTabs();renderSide();updateStatus();notify("文件已删除，当前编辑内容已保留");
}
async function restoreDeletedFiles() {
  const files=await api.ListDeletedFiles();
  const promise=dialog("恢复删除的文件",'<div id="deleted-list"></div>',[["关闭","cancel"]]);
  const list=$("#deleted-list");
  if(!files.length)list.textContent="没有可恢复的删除记录";
  for(const file of files){
    const button=document.createElement("button");button.className="file-item";
    button.textContent=file.originalPath+" · "+new Date(file.deletedAt).toLocaleString();
    button.onclick=()=>safely(async()=>{await flush();const doc=await api.RestoreDeletedFile(file.id);closeDialog();await addDocument(doc);if(folder)tree=await api.FolderTree(folder);renderSide();notify("已恢复："+doc.path);});
    list.append(button);
  }
  await promise;
}
async function windowList() {
  const windows=await api.Windows();const promise=dialog("MarkdownReader 窗口",'<div id="window-list"></div>',[["关闭","cancel"]]);
  for(const window of windows){const b=document.createElement("button");b.className="file-item";b.textContent=(window.current?"● ":"")+window.title;b.onclick=()=>safely(async()=>{closeDialog();await api.ActivateWindow(window.handle);});$("#window-list").append(b);}
  await promise;
}
function renderFolderSearch() {
  const c = $("#side-content");
  c.innerHTML =
    '<input id="folder-query" placeholder="搜索文件夹内容" aria-label="文件夹搜索"><button id="folder-search">搜索</button><div id="folder-results"></div>';
  $("#folder-search").onclick = () =>
    safely(async () => {
      if (!folder) {
        notify("请先打开文件夹");
        return;
      }
      const hits = await api.SearchFolder(folder, $("#folder-query").value);
      const r = $("#folder-results");
      r.innerHTML = "";
      for (const hit of hits) {
        const b = document.createElement("button");
        b.className = "search-hit";
        b.textContent = `${hit.path.split(/[\\/]/).at(-1)}:${hit.line} ${hit.text}`;
        b.title = hit.path;
        b.onclick = () =>
          safely(async () => {
            await addDocumentFromPath(hit.path);
            if (!source) await toggleSource();
            const start =
              content()
                .split("\n")
                .slice(0, hit.line - 1)
                .join("\n").length + (hit.line > 1 ? 1 : 0);
            $("#source").focus();
            $("#source").setSelectionRange(start, start + hit.text.length);
            $("#source").scrollTop = hit.line * settings.fontSize * 1.7;
          });
        r.append(b);
      }
      if (!hits.length) r.textContent = "没有匹配结果";
    });
  $("#folder-query").onkeydown = (e) => {
    if (e.key === "Enter") $("#folder-search").click();
  };
}
function flatTree(items) {
  return items.flatMap((x) => (x.dir ? flatTree(x.children || []) : [x]));
}
async function quickOpen() {
  const items = [
    ...flatTree(tree),
    ...(settings.recent || []).map((p) => ({
      path: p,
      name: p.split(/[\\/]/).at(-1),
    })),
  ];
  const unique = [...new Map(items.map((x) => [x.path, x])).values()];
  const promise = dialog(
    "快速打开",
    '<input id="quick-filter" placeholder="输入文件名…"><div id="quick-results"></div>',
    [["取消", "cancel"]],
  );
  const update = () => {
    const q = $("#quick-filter").value.toLowerCase();
    const c = $("#quick-results");
    c.innerHTML = "";
    for (const x of unique
      .filter((x) => x.path.toLowerCase().includes(q))
      .slice(0, 100)) {
      const b = document.createElement("button");
      b.type = "button";
      b.className = "quick-item";
      b.textContent = x.name;
      b.title = x.path;
      b.onclick = () => {
        closeDialog();
        safely(() => addDocumentFromPath(x.path));
      };
      c.append(b);
    }
  };
  $("#quick-filter").oninput = update;
  update();
  await promise;
}
let searchPaintFrame = 0;
function scheduleSearchHighlights() {
  cancelAnimationFrame(searchPaintFrame);
  searchPaintFrame = requestAnimationFrame(() => { if (!$("#findbar").hidden) findMatches(true); else clearSearchHighlights(); });
}
function clearSearchHighlights() {
  clearSourceSearchHighlights();
  globalThis.CSS?.highlights?.delete('markdownreader-search');
  globalThis.CSS?.highlights?.delete('markdownreader-search-current');
}
function paintSearchHighlights(matches) {
  clearSearchHighlights();
  if (currentComposition) return;
  if ($("#findbar").hidden) return;
  if (source) { paintSourceSearchHighlights($("#source"), matches, findPos); return; }
  if (!globalThis.CSS?.highlights || !globalThis.Highlight) return;
  const root = $("#editor .vditor-ir .vditor-reset");
  if (!root) return;
  const {text, nodes} = editableText(root), patches = diff.diff_main(content(), text);
  const point = offset => {
    offset = diff.diff_xIndex(patches, offset);
    for (const entry of nodes) if (offset <= entry.start + entry.node.length) return [entry.node, Math.max(0, offset - entry.start)];
    return [root, root.childNodes.length];
  };
  const all = new Highlight(), current = new Highlight();
  for (const match of matches) {
    if (!match[0].length) continue;
    const range = document.createRange();
    range.setStart(...point(match.index)); range.setEnd(...point(match.index + match[0].length));
    if (!range.collapsed) { all.add(range); if (match.index === findPos) current.add(range); }
  }
  current.priority = 1;
  CSS.highlights.set('markdownreader-search', all);
  CSS.highlights.set('markdownreader-search-current', current);
}
function findMatches(preserveNavigation = false) {
  const query = $("#find-query").value;
  if (!query) { $("#find-result").textContent = ""; clearSearchHighlights(); return []; }
  let re;
  try {
    re = searchPattern(query, {regex:$("#find-regex").checked, caseSensitive:$("#find-case").checked, wholeWord:$("#find-word").checked});
  } catch (e) {
    clearSearchHighlights();
    notify("正则表达式错误：" + e.message, true);
    return [];
  }
  const matches = [...content().matchAll(re)];
  paintSearchHighlights(matches);
  const current = preserveNavigation ? matches.findIndex(match => match.index === findPos) : -1;
  $("#find-result").textContent = current < 0 ? matches.length + " 处" : `${current + 1}/${matches.length}`;
  return matches;
}
async function findNext(previous = false) {
  const matches = findMatches();
  if (!matches.length) return;
  const [start, end] = currentSelection();
  let index = previous
    ? matches.findLastIndex((m) => m.index < start)
    : matches.findIndex(
        (m) => m.index >= end && m.index !== findPos,
      );
  if (index < 0) index = previous ? matches.length - 1 : 0;
  const m = matches[index];
  findPos = m.index;
  setMarkdownSelection(m.index, m.index + m[0].length, true);
  if (source) scrollSourceSelection($("#source"));
  else scrollRenderedSelection($("#editor .vditor-ir > .vditor-reset"));
  paintSearchHighlights(matches);
  $("#find-result").textContent = `${index + 1}/${matches.length}`;
}
async function replaceAll() {
  const matches = findMatches();
  if (!matches.length) return;
  const value = $("#replace-query").value;
  const result = replaceMatches(content(), matches, value, $("#find-regex").checked);
  applyTextEdit(0, content().length, result, true);
  findPos = -1;
  notify(`已替换 ${matches.length} 处`);
}
function showFind() {
  rememberSelection();
  const [start, end] = currentSelection();
  if (end > start) {
    $("#find-query").value = content().slice(start, end);
    findPos = -1;
  }
  $("#findbar").hidden = false;
  findMatches();
  $("#find-query").focus();
  $("#find-query").select();
}
async function exportHTML(plain = false, markdown = content()) {
  await configureParserMode();
  await prepareMathJax(cdn, !!settings.markdown.physics, settings.markdown.mathLineBreaks !== false);
  const area = $("#print-area");
  area.hidden = false;
  await Vditor.preview(area, markdown, {
    cdn,
    renderers: {
      renderSoftBreak: (_node, entering) => [entering ? (settings.markdown.keepSingleLineBreaks === false ? ' ' : '<br />\n') : '', Lute.WalkContinue],
    },
    mode: settings.theme === "Night" ? "dark" : "light",
    markdown: {
      sanitize: true,
      toc: true,
      mark: settings.markdown.mark,
      sub: settings.markdown.sub,
      sup: settings.markdown.sup,
      linkBase: active.baseURL || "",
    },
    math: {
      engine: "MarkdownReader",
      inlineDigit: !!settings.markdown.inlineMathDigit,
      mathJaxOptions: { svg: { fontCache: "none" } },
    },
    hljs: { lineNumber: settings.lineNumbers },
    theme: { current: "light", path: cdn + "/dist/css/content-theme" },
  });
  const latexMath=plain&&settings.markdown.plainHTMLMathLatex;
  if(latexMath){
    for(const node of area.querySelectorAll('.language-math')){
      const tex=node.getAttribute('data-math')??node.textContent;
      const display=node.tagName==='DIV',replacement=display?document.createElement('div'):document.createTextNode('');
      replacement.textContent=display?'$$\n'+tex+'\n$$':'$'+tex+'$';
      node.replaceWith(replacement);
    }
  }else await renderDocumentMath(area,mathOptions());
  new PunctuationView().apply(area,!!settings.markdown.smartRender,{editing:false});
  await patchImages(area);
  await document.fonts.ready;
  await new Promise((r) => setTimeout(r, 700));
  let css = "";
  if (!plain) {
    for (const sheet of document.styleSheets) {
      try {
        let text = Array.from(sheet.cssRules)
          .map((r) => r.cssText)
          .join("\n");
        for (const match of [...text.matchAll(/url\(["']?([^"')]+)["']?\)/g)]) {
          if (match[1].startsWith("data:")) continue;
          const url = new URL(match[1], sheet.href || document.baseURI);
          if (url.origin !== location.origin) continue;
          if (!exportedAssetCache.has(url.href)) {
            const response = await fetch(url.href);
            if (!response.ok) throw Error("导出资源缺失：" + url.href);
            const blob = await response.blob();
            const data = await new Promise((resolve) => {
              const reader = new FileReader();
              reader.onload = () => resolve(reader.result);
              reader.readAsDataURL(blob);
            });
            exportedAssetCache.set(url.href, data);
          }
          text = text
            .split(match[0])
            .join(`url("${exportedAssetCache.get(url.href)}")`);
        }
        css += text;
      } catch {}
    }
  }
  const clone = area.cloneNode(true);
  clone.querySelectorAll("button,.vditor-copy").forEach((x) => x.remove());
  for (const img of clone.querySelectorAll("img")) {
    if (img.src.startsWith(location.origin + "/files/")) {
      try {
        const blob = await (await fetch(img.src)).blob();
        img.src = await new Promise((r) => {
          const reader = new FileReader();
          reader.onload = () => r(reader.result);
          reader.readAsDataURL(blob);
        });
      } catch {}
    }
  }
  return `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>${esc(active.name)}</title>${plain ? "" : `<style>${css}\nbody{display:block;padding:32px;background:white;color:#222}article{max-width:860px;margin:auto}</style>`}<body><article class="vditor-reset">${clone.innerHTML}</article></body></html>`;
}
async function doExport(format, repeat = false) {
  if (exportInProgress) throw Error("已有导出任务正在进行");
  exportInProgress = true;
  document.body.classList.add("export-busy");
  const originalTitle = document.title;
  document.title = active.name;
  const printStyle = document.createElement("style");
  const pdf = settings.pdf;
  printStyle.textContent = `@page{size:${pdf.width}mm ${pdf.height}mm;margin:${pdf.margin}mm} ${pdf.breakH1 ? "@media print{#print-area h1:not(:first-child){break-before:page}}" : ""}`;
  document.head.append(printStyle);
  try {
    await flush();
    const preset=settings.customExports?.find(item=>'custom:'+item.id===format);
    const program=preset?.kind==='program';
    const html = program?'':await exportHTML(format === "html-plain");
    if(!program){document.body.classList.add("exporting");await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));}
    const path = await api.ExportDialog(active.id, format, html, repeat);
    if (path||(program&&preset.outputMode==='none')) {
      acceptSettings(await api.GetSettings());
      notify(program&&preset.outputMode==='none'?'已执行：'+preset.name:"已导出：" + path);
      if(program&&preset.showOutput)await dialog('导出命令输出',`<pre style="white-space:pre-wrap;max-height:50vh;overflow:auto">${esc(await api.GetExportOutput()||'转换程序没有输出消息。')}</pre>`);
    }
  } finally {
    exportInProgress = false;
    document.body.classList.remove("export-busy");
    document.title = originalTitle;
    printStyle.remove();
    document.body.classList.remove("exporting");
    $("#print-area").hidden = true;
  }
}
async function prefs(page = "general") {
  let exportDraft = structuredClone(settings.customExports || []);
  const readExportDraft = () => {
    for(const [index,element] of [...document.querySelectorAll('#custom-export-list article')].entries()) {
      for(const field of element.querySelectorAll('[data-preset-field]')) exportDraft[index][field.dataset.presetField]=field.type==='checkbox'?field.checked:field.dataset.presetField==='arguments'?field.value.split(/\r?\n/).filter(value=>value!==''):field.value;
    }
  };
  const renderExportDraft = () => {
    $('#custom-export-list').innerHTML=exportDraft.map((preset,index)=>{
      const program=preset.kind==='program';
      return `<article data-preset-index="${index}"><label>名称<input data-preset-field="name" value="${esc(preset.name)}"></label><label>转换方式<select data-preset-field="kind"><option value="pandoc" ${program?'':'selected'}>随包 Pandoc</option><option value="program" ${program?'selected':''}>自定义转换程序</option></select></label>${program?`<label>程序路径或名称<input data-preset-field="program" value="${esc(preset.program||'')}" placeholder="例如 tools/pandoc.exe"></label><label>输出方式<select data-preset-field="outputMode"><option value="file" ${preset.outputMode==='stdout'?'':'selected'}>程序写入文件（参数使用 \${outputPath}）</option><option value="stdout" ${preset.outputMode==='stdout'?'selected':''}>标准输出写入文件</option></select></label><label class="check"><input type="checkbox" data-preset-field="showOutput" ${preset.showOutput?'checked':''}>成功后显示命令输出</label>`:`<label>Pandoc 输出格式<input data-preset-field="writer" value="${esc(preset.writer)}" placeholder="例如 gfm、docx、html5"></label>`}<label>文件扩展名<input data-preset-field="extension" value="${esc(preset.extension)}" placeholder="例如 md"></label><label>参数（每行一个，空格原样保留）<textarea data-preset-field="arguments" rows="4">${esc((preset.arguments||[]).join('\n'))}</textarea></label>${program?'<p class="muted">当前内容也通过标准输入传入。${inputPath} 是内容快照，${outputPath} 是本次转换输出路径。相对程序路径从 MarkdownReader 目录解析。</p>':''}<div><button type="button" data-preset-action="up" ${index===0?'disabled':''}>上移</button><button type="button" data-preset-action="down" ${index===exportDraft.length-1?'disabled':''}>下移</button><button type="button" data-preset-action="remove">移除</button></div></article>`;
    }).join('');
    for(const [index,preset] of exportDraft.entries()){
      const article=document.querySelector(`#custom-export-list article[data-preset-index="${index}"]`),label=document.createElement('label');
      label.innerHTML='<span>导出后打开输出目录</span><select data-preset-field="afterExport"><option value="">使用全局设置</option><option value="open-folder">打开输出目录</option><option value="none">不打开目录</option></select>';
      label.querySelector('select').value=preset.afterExport||'';
      label.hidden=preset.kind==='program'&&preset.outputMode==='none';
      article.querySelector('label').after(label);
    }
    for(const [index,preset] of exportDraft.entries())if(preset.kind==='program'){
      const label=document.createElement('label');
      label.innerHTML=`命令输出编码<select data-preset-field="outputEncoding"><option value="auto">自动（Unicode / GB18030）</option>${encodingOptions.map(option=>`<option value="${esc(option.name)}" ${preset.outputEncoding===option.name?'selected':''}>${esc(option.label)}</option>`).join('')}</select>`;
      document.querySelector(`#custom-export-list article[data-preset-index="${index}"] [data-preset-field="outputMode"]`).closest('label').after(label);
      const article=document.querySelector(`#custom-export-list article[data-preset-index="${index}"]`),mode=article.querySelector('[data-preset-field="outputMode"]');
      mode.add(new Option('程序写入文件夹（参数使用 ${outputPath}）','folder'));
      mode.add(new Option('仅执行命令，不生成文件','none'));mode.value=preset.outputMode||'file';
      article.querySelector('[data-preset-field="extension"]').closest('label').hidden=['none','folder'].includes(mode.value);
    }
  };
  const tabs = [
    ["general", "通用"],
    ["appearance", "外观"],
    ["editor", "编辑器"],
    ["image", "图像"],
    ["markdown", "Markdown"],
    ["export", "导出"],
    ["shortcuts", "快捷键"],
  ];
  const row = (label, field, type = "checkbox", value = settings[field]) =>
    type === "checkbox"
      ? `<label class="check"><input type="checkbox" name="${field}" ${value ? "checked" : ""}>${label}</label>`
      : `<label>${label}<input name="${field}" type="${type}" value="${esc(value)}"></label>`;
  let html = "";
  if (page === "general")
    html = `<label>启动时<select name="startup"><option value="new" ${settings.startup === "new" ? "selected" : ""}>打开新文件</option><option value="last" ${settings.startup === "last" ? "selected" : ""}>恢复最近文件</option></select></label>${row("自动保存", "autoSave")}${row("切换文档时自动保存", "saveOnSwitch")}<button type="button" id="show-recovery">恢复未保存的草稿</button><p class="muted">设置与草稿保存在程序旁的 data 目录中。</p>`;
  if (page === "appearance")
    html = `${row("字体大小（10–40）", "fontSize", "number")}${row("阅读速度（字/分钟）", "readingSpeed", "number")}<label>主题<select name="theme">${["Github", "Newsprint", "Night", "Pixyll", "Whitey"].map((t) => `<option ${settings.theme === t ? "selected" : ""}>${t}</option>`).join("")}</select></label>${row("专注模式", "focus")}${row("打字机模式", "typewriter")}`;
  if (page === "editor")
    html = `${row("默认缩进（1–8）", "indent", "number")}<label>新文件换行符<select name="eol"><option ${settings.eol === "LF" ? "selected" : ""}>LF</option><option ${settings.eol === "CRLF" ? "selected" : ""}>CRLF</option></select></label>${row("保存时添加末尾换行", "finalNewline")}${row("匹配括号和引号", "pair")}${row("匹配 Markdown 字符", "markdownPair")}${row("默认启用拼写检查", "spellcheck")}<label>默认拼写语言<select name="spellLanguage"><option value="en-US" ${settings.spellLanguage!=="en-GB"?"selected":""}>English (US)</option><option value="en-GB" ${settings.spellLanguage==="en-GB"?"selected":""}>English (UK)</option></select></label>${row("代码块行号", "lineNumbers")}${row("代码块自动换行", "codeWrap")}`;
  if (page === "markdown") {
    const m = settings.markdown;
    html = `${row("显示 &lt;br/&gt;", "markdown.showBRMarkers", "checkbox", m.showBRMarkers !== false)}${row("保留单换行符", "markdown.keepSingleLineBreaks", "checkbox", m.keepSingleLineBreaks !== false)}${row("高亮", "markdown.mark", "checkbox", m.mark)}${row("上标", "markdown.sup", "checkbox", m.sup)}${row("下标", "markdown.sub", "checkbox", m.sub)}${row("首行缩进", "paragraphIndent")}${row("渲染时转换智能标点", "markdown.smartRender", "checkbox", !!m.smartRender)}${row("智能引号（输入时）", "markdown.smartQuotes", "checkbox", m.smartQuotes)}${row("智能破折号（源码输入）", "markdown.smartDashes", "checkbox", m.smartDashes)}<label>标题样式<select name="markdown.headingStyle"><option value="atx" ${m.headingStyle === "atx" ? "selected" : ""}>atx (#)</option><option value="setext" ${m.headingStyle === "setext" ? "selected" : ""}>setext (=== / ---)</option></select></label><label>无序列表标记<select name="markdown.listMarker">${["-", "+", "*"].map((x) => `<option ${m.listMarker === x ? "selected" : ""}>${x}</option>`).join("")}</select></label>`;
  }
  if (page === "markdown")
    html = row("严格模式", "markdown.strictMode", "checkbox", settings.markdown.strictMode !== false) + row("允许行内公式以数字开头", "markdown.inlineMathDigit", "checkbox", !!settings.markdown.inlineMathDigit) + row("启用 physics 包", "markdown.physics", "checkbox", !!settings.markdown.physics) + row("自动添加公式序号", "markdown.mathNumbering", "checkbox", !!settings.markdown.mathNumbering) + row("支持使用双反斜杠和 \\newline 换行", "markdown.mathLineBreaks", "checkbox", settings.markdown.mathLineBreaks !== false) + `<label>当复制或导出为无格式 HTML 时<select name="markdown.plainHTMLMathLatex"><option value="false" ${!settings.markdown.plainHTMLMathLatex ? "selected" : ""}>使用 SVG</option><option value="true" ${settings.markdown.plainHTMLMathLatex ? "selected" : ""}>使用 LaTeX 代码</option></select></label>` + html;
  if (page === "image")
    html = `<label>插入图片时…<select name="imageInsertAction">${imageInsertPresets.map(([value,label])=>`<option value="${value}" ${selectedImageInsertPreset(settings)===value?"selected":""}>${label}</option>`).join("")}</select></label>${row("对本地位置的图片应用上述规则", "applyLocalImageRules")}${row("对网络位置的图片应用上述规则", "applyRemoteImageRules")}${row("图片复制目录", "imageFolder", "text")}<button type="button" id="image-folder-browse">选择图片目录…</button><p class="muted" id="image-folder-help">可使用绝对路径或相对于文档的路径。\${filename} 使用当前文档文件名（不含扩展名）。</p>${row("允许根据 YAML 设置自动上传图片", "allowYAMLImageUpload")}${row("优先使用相对路径", "relativeImages")}<p class="muted">插入本地图片前请保存文档。</p><h3>上传服务</h3><label>上传服务<select name="imageUploader"><option value="none" ${!['custom','picgo-core','picgo-app'].includes(settings.imageUploader)?'selected':''}>无</option><option value="picgo-core" ${settings.imageUploader==='picgo-core'?'selected':''}>PicGo-Core (command line)</option><option value="picgo-app" ${settings.imageUploader==='picgo-app'?'selected':''}>PicGo (app)</option><option value="custom" ${settings.imageUploader==='custom'?'selected':''}>Custom Command</option></select></label>${row("命令", "imageUploadCommand", "text")}${row("PicGo 程序路径", "imageUploaderProgram", "text")}<button type="button" id="image-uploader-browse">选择 PicGo 程序…</button><p class="muted" id="image-upload-help">图片路径按顺序追加为参数，标准输出最后的对应行作为图片地址。支持 \${filename} 和 \${filepath}。</p><button type="button" id="image-upload-test">验证图片上传选项</button><button type="button" id="image-upload-output">查看上次上传输出</button><pre id="image-upload-result"></pre>`;
  if (page === "export")
    html = `${row("默认导出目录", "exportFolder", "text")}${row("导出后打开文件位置", "openAfterExport")}<h3>PDF 页面</h3>${row("宽度（毫米）", "pdf.width", "number", settings.pdf.width)}${row("高度（毫米）", "pdf.height", "number", settings.pdf.height)}${row("页边距（毫米）", "pdf.margin", "number", settings.pdf.margin)}${row("按一级标题分页", "pdf.breakH1", "checkbox", settings.pdf.breakH1)}${row("页首", "pdf.header", "text", settings.pdf.header)}${row("页尾", "pdf.footer", "text", settings.pdf.footer)}<p class="muted">页首、页尾支持 {page}、{pages}、{title}、{date}。</p>`;
  if (page === "shortcuts")
    html =
      '<p class="muted">输入如 Ctrl+Shift+B。留空使用默认快捷键。</p>' +
      [...keys]
        .filter(([, id]) => !id.includes(":"))
        .map(
          ([key, id]) =>
            `<label>${esc(id)}（${esc(key)}）<input name="key:${esc(id)}" value="${esc(settings.shortcuts?.[id] || "")}"></label>`,
        )
        .join("");
  if(page==='export') html += '<h3>自定义导出</h3><div id="custom-export-list"></div><button type="button" id="custom-export-add">添加导出项</button><p class="muted">支持 ${currentPath}、${currentFolder}、${currentFileName}、${currentFileFullName}、${outputFileName}、${outputFileFullName}、${today}，以及 YAML 前言中的 ${title}、${author}、${a.b} 等字段。输出路径由保存对话框指定。</p>';
  const promise = dialog(
    "偏好设置",
    `<div class="prefs"><nav>${tabs.map(([id, n]) => `<button type="button" data-pref="${id}" class="${id === page ? "active" : ""}">${n}</button>`).join("")}</nav><section>${html}</section></div>`,
    [
      ["保存", "ok"],
      ["取消", "cancel"],
    ],
  );
  if(page==='export') {
    renderExportDraft();
    $('#custom-export-add').onclick=()=>{readExportDraft();exportDraft.push({id:crypto.randomUUID(),name:'自定义导出',writer:'gfm',extension:'md',arguments:[]});renderExportDraft();};
    $('#custom-export-list').onclick=event=>{
      const button=event.target.closest('[data-preset-action]');if(!button)return;
      readExportDraft();const index=Number(button.closest('article').dataset.presetIndex),action=button.dataset.presetAction;
      if(action==='remove')exportDraft.splice(index,1);
      else {const target=index+(action==='up'?-1:1);if(target>=0&&target<exportDraft.length)[exportDraft[index],exportDraft[target]]=[exportDraft[target],exportDraft[index]];}
      renderExportDraft();
    };
    $('#custom-export-list').onchange=event=>{if(['kind','outputMode'].includes(event.target.dataset.presetField)){readExportDraft();renderExportDraft();}};
  }
  document.querySelectorAll("[data-pref]").forEach(
    (b) =>
      (b.onclick = () => {
        closeDialog();
        safely(() => prefs(b.dataset.pref));
      }),
  );
  if($('#image-folder-browse')){
    const select=document.querySelector('[name="imageInsertAction"]'),field=document.querySelector('[name="imageFolder"]'),browse=$('#image-folder-browse'),help=$('#image-folder-help');
    const show=()=>{const custom=select.value==='copy';field.closest('label').hidden=!custom;browse.hidden=!custom;help.hidden=!custom;};
    select.onchange=show;show();
    browse.onclick=()=>safely(async()=>{const path=await api.ChooseGlobalImageFolder();if(path)field.value=path;});
  }
  if($('#image-upload-output')){
    const test=$('#image-upload-test'),service=document.querySelector('[name="imageUploader"]'),command=document.querySelector('[name="imageUploadCommand"]'),output=$('#image-upload-result');
    const program=document.querySelector('[name="imageUploaderProgram"]'),browse=$('#image-uploader-browse');
    const availability=()=>{const custom=service.value==='custom',picgo=['picgo-core','picgo-app'].includes(service.value);command.closest('label').hidden=!custom;program.closest('label').hidden=!picgo;browse.hidden=!picgo;$('#image-upload-help').textContent=custom?'图片路径按顺序追加为参数；支持 ${filename} 和 ${filepath}。':service.value==='picgo-core'?'选择 PicGo-Core 程序，以 upload 命令上传所选本地图片。':service.value==='picgo-app'?'使用本机 PicGo-Server（127.0.0.1:36677）；服务未运行时尝试启动指定程序。':'选择图片上传服务。';test.disabled=service.value==='none'||(custom?!command.value.trim():service.value==='picgo-core'&&!program.value.trim());};
    browse.onclick=()=>safely(async()=>{const path=await api.ChooseImageUploaderProgram();if(path)program.value=path;availability();});program.oninput=availability;
    service.onchange=availability;command.oninput=availability;availability();
    test.onclick=async()=>{test.disabled=true;output.textContent='正在验证上传选项…';try{const result=await api.TestImageUploader(service.value,service.value==='custom'?command.value:program.value);output.textContent='验证成功\n'+result.output;}catch(error){output.textContent=String(error)+'\n'+await api.GetImageUploadOutput();}finally{availability();}};
    $('#image-upload-output').onclick=()=>safely(async()=>{output.textContent=await api.GetImageUploadOutput()||'暂无上传输出';});
  }
  if ($("#show-recovery"))
    $("#show-recovery").onclick = () => {
      closeDialog();
      safely(showRecovery);
    };
  const result = await promise;
  if (result.value !== "ok") return;
  if(page==='export') {readExportDraft();settings.customExports=exportDraft;}
  for (const field of $("#modal-content").querySelectorAll("[name]")) {
    const name = field.name;
    if (name.startsWith("key:")) {
      settings.shortcuts ??= {};
      settings.shortcuts[name.slice(4)] = field.value;
    } else {
      const parts = name.split(".");
      const owner = parts.length === 2 ? settings[parts[0]] : settings;
      owner[parts.at(-1)] =
        name === "markdown.plainHTMLMathLatex"
          ? field.value === "true"
          : field.type === "checkbox"
          ? field.checked
          : field.type === "number"
            ? Number(field.value)
            : field.value;
    }
  }
  if(page==='image')Object.assign(settings,imageInsertPresetSettings(settings.imageInsertAction));
  await persist();
  buildMenus();
  if (!source) await mountEditor();
  notify("设置已保存");
}
async function showRecovery() {
  const boot = await api.Bootstrap();
  recovery = boot.recovery || [];
  if (!recovery.length) {
    notify("没有可恢复的草稿");
    return;
  }
  const promise = dialog("恢复未保存的草稿", '<div id="recovery-list"></div>', [
    ["关闭", "cancel"],
  ]);
  for (const d of recovery) {
    const b = document.createElement("button");
    b.type = "button";
    b.textContent =
      d.name +
      " — " +
      (d.path || "未保存") +
      "（" +
      d.content.length +
      " 字符）";
    b.onclick = () =>
      safely(async () => {
        closeDialog();
        await addDocument(await api.RestoreDocument(d));
      });
    $("#recovery-list").append(b);
  }
  await promise;
}
async function run(id) {
  if(!source){punctuationView.restore();rememberSelection();}
  pendingInput = null;
  if (active) active.historyGroup = null;
  if (id !== "undo" && id !== "redo") rememberHistoryCursor();
  if(id.startsWith("reopen:"))return reopenEncoding(id.slice(7));
  if(id.startsWith("saveencoding:")){
    await flush();Object.assign(active,await api.SetDocumentEncoding(active.id,id.slice(13)));
    renderTabs();updateStatus();return notify("保存编码已设为 "+active.encoding+"；保存后生效");
  }
  if (id.startsWith("link:")) return linkCommand(id.slice(5), false);
  if(id==='image-upload'||id==='image-upload-all'){
    const selected=id==='image-upload'?currentLink(true):null;
    if(id==='image-upload'&&!selected)return notify('请先选中图片',true);
    const owner=active,original=content(),references=selected?[selected.destination]:[...new Set(documentImages(original).map(image=>image.destination))];
    await flush();
    if(active!==owner||content()!==original)throw Error('文档已变化，请重新上传图片');
    const result=await api.UploadImages(owner.id,references);
    if(active!==owner||content()!==original||result.original!==original)throw Error('上传已结束，文档已变化，未替换图片地址；可在图像设置查看返回结果');
    if(selected){const destination=result.references[selected.destination];if(destination)applyTextEdit(selected.start,selected.end,replaceImageDestination(original,selected,destination));}
    else {const value=replaceImageDestinations(original,result.references);if(value!==original)applyTextEdit(0,original.length,value,true);}
    lastLinkContext=null;
    return notify(`已上传 ${result.uploaded} 个图片文件`+(result.skipped?`；保留 ${result.skipped} 个非本地引用`:''));
  }
  if(id==='image-files-copy'||id==='image-files-move') {
    const images=documentImages(content()),references=[...new Set(images.map(image=>image.destination))],labels={};
    for(const image of images)if(!(image.destination in labels))labels[image.destination]=image.label;
    if(!references.length)return notify("文档中没有图片",true);
    await flush();const owner=active,original=content();
    const moving=id==='image-files-move';
    const result=await (moving?api.MoveImages(owner.id,references):api.CopyImages(owner.id,references,labels));if(!result)return;
    if(active!==owner||content()!==original||result.original!==original){
      if(result.token)await api.SetImageCopyState(owner.id,result.token,false);
      throw Error("文档已变化，图片文件操作已撤销，请重试");
    }
    const value=replaceImageDestinations(original,result.references);
    if(value!==original){
      applyTextEdit(0,original.length,value,true);
      if(result.token)owner.historyEffects[owner.historyIndex]={imageCopy:result.token};
      lastLinkContext=null;
    }
    return notify(`已${moving?'移动':'复制'} ${result.copied} 个图片文件`+(result.skipped?`；保留 ${result.skipped} 个未处理引用`:''));
  }
  if(id==='image-file-copy'||id==='image-file-move') {
    const link=currentLink(true);if(!link)return notify("请先选中图片",true);
    await flush();const owner=active,original=content();
    const moving=id==='image-file-move';
    const aliases=moving?await imageMoveAliases(documentImages(original).map(image=>image.destination),link.destination,reference=>api.ImageLocation(owner.id,reference)):null;
    if(active!==owner||content()!==original)throw Error('文档已变化，请重新选择图片');
    const result=await (moving?api.MoveImage(owner.id,link.destination):api.CopyImage(owner.id,link.destination,link.label));if(!result)return;
    if(active!==owner||content()!==original||result.original!==original){
      if(result.token)await api.SetImageCopyState(owner.id,result.token,false);
      throw Error("文档已变化，图片文件操作已撤销，请重试");
    }
    if(moving){
      const value=replaceImageDestinations(original,movedImageReferences(aliases,result.reference));
      if(value===original&&result.token){await api.SetImageCopyState(owner.id,result.token,false);throw Error('无法更新图片路径，移动已撤销');}
      if(value!==original){applyTextEdit(0,original.length,value);if(result.token)owner.historyEffects[owner.historyIndex]={imageCopy:result.token};lastLinkContext=null;}
      return;
    }
    const replacement=replaceImageDestination(original,link,result.reference);
    if(replacement!==original.slice(link.start,link.end)){
      applyTextEdit(link.start,link.end,replacement);
      if(result.token)owner.historyEffects[owner.historyIndex]={imageCopy:result.token};
      const updated=documentImages(content()).find(l=>l.start===link.start);
      lastLinkContext=updated?{id:owner.id,content:content(),link:updated}:null;
    }
    return;
  }
  if (id==='image-root'||id==='image-copy-folder'||id==='image-upload-rule') {
    await flush();const owner=active;
    const result=await (id==='image-root'?api.ChooseImageRoot(owner.id):id==='image-upload-rule'?api.SetImageUploadRule(owner.id):api.ChooseImageCopyFolder(owner.id));if(!result)return;
    if(active!==owner||content()!==result.original)throw Error("文档已变化，请重新设置图片目录");
    if(result.content===result.original)return;
    applyTextEdit(0,result.original.length,result.content,true);
    await flush();await patchImages($("#editor"));return;
  }
  if (id.startsWith("image-scale:")) {
    const link=currentLink(true);if(!link)return notify("请先选中图片",true);
    const original=content(),replacement=scaledImage(original,link,Number(id.slice(12)));
    if(replacement===original.slice(link.start,link.end))return;
    applyTextEdit(link.start,link.end,replacement);
    const updated=documentImages(content()).find(l=>l.start===link.start);
    lastLinkContext=updated?{id:active.id,content:content(),link:updated}:null;
    return;
  }
  if (id.startsWith("image:")) return linkCommand(id.slice(6), true);
  if (id.startsWith("table:")) return tableCommand(id.slice(6));
  if (!ready && id !== "about") return;
  if (id.startsWith("openpath:")) return addDocumentFromPath(id.slice(9));
  if (id.startsWith("export:")) return doExport(id.slice(7));
  if (id.startsWith("preferences:")) return prefs(id.slice(12));
  if (id.startsWith("side:")) return showSidebar(id.slice(5));
  if (id.startsWith("theme:")) {
    settings.theme = id.slice(6);
    return persist();
  }
  if (id.startsWith("eol:")) {
    active.eol = id.slice(4);
    active.dirty = true;
    changed(content(), false);
    return flush();
  }
  if (id.startsWith("heading:")) {
    const level = Number(id.slice(8));
    if (!source && level && settings.markdown.headingStyle !== "setext") {
      if (toolbar("headings")) {
        const b = $(`#editor [data-type="h${level}"]`);
        if (b) {
          b.click();
          changed(readEditor());
          return;
        }
      }
    }
    return editSource((v, a, b) => {
      const [start, end] = lineBounds(v, a, b);
      const bare = v.slice(start, end).replace(/^#{1,6}\s+/gm, "");
      if (
        settings.markdown.headingStyle === "setext" &&
        (level === 1 || level === 2)
      ) {
        return {
          start,
          end,
          text:
            bare +
            "\n" +
            (level === 1 ? "=" : "-").repeat(Math.max(3, bare.length)),
        };
      }
      return {
        start,
        end,
        text: v
          .slice(start, end)
          .replace(/^#{1,6}\s+/gm, "")
          .split("\n")
          .map((l) => (level ? "#".repeat(level) + " " : "") + l)
          .join("\n"),
      };
    });
  }
  switch (id) {
    case "new":
      return addDocument(await api.NewDocument());
    case "window":
      return api.NewWindow("");
    case "open":
      await flush();
      return addDocument(await api.OpenDialog());
    case "openencoding": return openWithEncoding();
    case "folder": {
      const p = await api.OpenFolderDialog();
      if (p) {
        folder = p;
        tree = await api.FolderTree(p);
        showSidebar("tree");
      }
      return;
    }
    case "quick":
      return quickOpen();
    case "save":
      return save();
    case "saveas":
      return save(true);
    case "saveall": {
      const old = active;
      for (const d of docs) {
        if (d.dirty) {
          await switchDoc(d);
          if (!(await save())) return;
        }
      }
      if (old) await switchDoc(old);
      return;
    }
    case "close":
      return closeDocument();
    case "renamefile": return renameCurrentFile();
    case "deletefile": return deleteCurrentFile();
    case "restorefile": return restoreDeletedFiles();
    case "nextwindow": return api.CycleWindow(false);
    case "prevwindow": return api.CycleWindow(true);
    case "windows": return windowList();
    case "import":
      return addDocument(await api.ImportDialog());
    case "properties": {
      if (!active.path) return notify("请先保存文档");
      const p = await api.Properties(active.path);
      return dialog(
        "文档属性",
        `<p>${esc(p.path)}</p><p>${p.size} 字节</p><p>修改时间：${esc(p.modified)}</p><p>${esc(active.encoding)} · ${active.eol}</p>`,
      );
    }
    case "reveal":
      if (active.path) return api.Reveal(active.path);
      return notify("请先保存文档");
    case "showfile":
      if (active.path) {
        folder = active.path.replace(/[\\/][^\\/]+$/, "");
        tree = await api.FolderTree(folder);
        return showSidebar("tree");
      }
      return;
    case "preferences":
      return prefs();
    case "exportlast":
    case "exportoverwrite":
      if (!settings.lastExport) return notify("尚无上一次导出设置");
      return doExport(settings.lastExport, id === "exportoverwrite");
    case "print":
      try {
        await exportHTML();
        document.body.classList.add("exporting");
        window.print();
      } finally {
        document.body.classList.remove("exporting");
        $("#print-area").hidden = true;
      }
      return;
    case "source":
      return toggleSource();
    case "sidebar":
      showSide = !showSide;
      $("#sidebar").hidden = !showSide;
      if (showSide) renderSide();
      return;
    case "focus":
      settings.focus = !settings.focus;
      return persist();
    case "typewriter":
      settings.typewriter = !settings.typewriter;
      return persist();
    case "status":
      $("#status").hidden = !$("#status").hidden;
      return;
    case "fullscreen":
      fullscreen = !fullscreen;
      return api.ToggleFullscreen(fullscreen);
    case "devtools":
      return openDeveloperTools();
    case "topmost":
      topmost = !topmost;
      return api.ToggleAlwaysOnTop(topmost);
    case "zoomin":
      zoom = Math.min(2.5, zoom + 0.1);
      applyAppearance();
      return;
    case "zoomout":
      zoom = Math.max(0.5, zoom - 0.1);
      applyAppearance();
      return;
    case "zoomreset":
      zoom = 1;
      applyAppearance();
      return;
    case "nextdoc":
      return switchDoc(docs[(docs.indexOf(active) + 1) % docs.length]);
    case "undo":
    case "redo":
      {
        const owner=active;
        const navigation=historyNavigation.catch(()=>{}).then(async()=>{
        if(active!==owner)return;
        restoreSelection();
        rememberHistoryCursor();
        const history = active.history || [];
        const target = (active.historyIndex || 0) + (id === "undo" ? -1 : 1);
        if (target >= 0 && target < history.length) {
          const previousIndex=owner.historyIndex,previousContent=owner.content;
          const effect=active.historyEffects?.[id==='undo'?active.historyIndex:target];
          if(effect?.imageCopy)await api.SetImageCopyState(active.id,effect.imageCopy,id==='redo');
          if(active!==owner||owner.historyIndex!==previousIndex||owner.content!==previousContent){
            if(effect?.imageCopy)await api.SetImageCopyState(owner.id,effect.imageCopy,id==='undo');
            throw Error("文档已变化，请重新执行撤销或重做");
          }
          active.historyIndex = target;
          changed(history[target], false);
          if (source) $("#source").value = history[target];
          else {
            editor.setValue(history[target]);
            captureRenderedBaseline();
            await patchImages($("#editor"));
          }
          if(active!==owner)return;
          lastRange = null;
          const cursor = active.historySelections?.[target] || [0, 0];
          setMarkdownSelection(...cursor.map(offset => Math.min(offset, content().length)));
        }
        });
        historyNavigation=navigation;return navigation;
      }
    case "cut":
      restoreSelection();
      document.execCommand("cut");
      if (source) changed($("#source").value);
      else changed(readEditor());
      return;
    case "copy":
      restoreSelection();
      document.execCommand("copy");
      return;
    case "copymd":
      await writeClipboard(selection() || content());
      return notify("已复制 Markdown");
    case "copytext":
    case "copysimple":
      await writeClipboard(
        selection() || $("#editor").innerText,
      );
      return notify("已复制纯文本");
    case "copyhtml": {
      const html = await exportHTML(true,selection() || content());
      const fragment = new DOMParser().parseFromString(html,"text/html").querySelector('article').innerHTML;
      await writeClipboard(fragment);
      return notify("已复制 HTML");
    }
    case "paste": {
      restoreSelection();
      const target = {owner:active, text:content(), position:currentSelection()};
      { const serial = pasteSerial;
        if (document.execCommand('paste') || pasteSerial !== serial) return;
      }
      if (navigator.clipboard.read) {
        let items;
        try { items = await navigator.clipboard.read(); } catch { /* Fall back to desktop plain text. */ }
        for (const item of items || []) {
          const type = item.types.find(type => type.startsWith('image/'));
          if (type) return pasteImageFile(await item.getType(type), target);
        }
      }
      return insert(await window.runtime.ClipboardGetText());
    }
    case "pastetext":
      return insert(await window.runtime.ClipboardGetText());
    case "delete":
      restoreSelection();
      document.execCommand("delete");
      if (source) changed($("#source").value);
      else changed(readEditor());
      return;
    case "selectall":
      return selectRange("all");
    case "selectblock":
      return selectRange("block");
    case "selectline":
      return selectRange("line");
    case "selectformat":
      return selectRange("format");
    case "selectword":
      return selectRange("word");
    case "deleteblock":
      return selectRange("block", true);
    case "deleteline":
      return selectRange("line", true);
    case "deleteformat":
      return selectRange("format", true);
    case "deleteword":
      return selectRange("word", true);
    case "start":
    case "end": {
      const offset = id === "start" ? 0 : content().length;
      setMarkdownSelection(offset);
      if (source)
        $("#source").scrollTop = id === "start" ? 0 : $("#source").scrollHeight;
      else {
        const viewport = $("#editor .vditor-ir > .vditor-reset");
        viewport.scrollTop = id === "start" ? 0 : viewport.scrollHeight;
      }
      return;
    }
    case "scrollselection":
      if (source) scrollSourceSelection($("#source"));
      else {
        if (!$("#editor").contains(getSelection()?.anchorNode)) restoreSelection();
        scrollRenderedSelection($("#editor .vditor-ir > .vditor-reset"));
      }
      return;
    case "bold":
    case "italic":
    case "strike":
    case "inline-code":
      if (!source && toolbar(id)) return;
      return wrap(
        { bold: "**", italic: "*", strike: "~~", "inline-code": "`" }[id],
      );
    case "underline":
      return wrap("<u>", "</u>");
    case "inlinemath":
      return wrap("$");
    case "mark":
      return wrap("==");
    case "sup":
      return wrap("^");
    case "sub":
      return wrap("~");
    case "comment":
      return wrap("<!-- ", " -->");
    case "clearstyle":
      return editSource((v, a, b) => ({
        start: a,
        end: b,
        text: v
          .slice(a, b)
          .replace(/(\*\*|__|~~|==|`|\*|_)/g, "")
          .replace(/<\/?(?:u|sup|sub|mark)>/g, ""),
      }));
    case "link": {
      return editLink(false);
    }
    case "imageurl": return editLink(true);
    case "imagesreload": {
      await patchImages($("#editor"));
      for (const image of $("#editor").querySelectorAll("img")) {
        await fetch(image.src, {cache:"reload"});
        const url=image.src; image.removeAttribute("src"); image.src=url;
      }
      return notify("已重新加载图片");
    }
    case "image": {
      const owner=active,text=content(),position=currentSelection();
      await flush();
      if(active!==owner||content()!==text)throw Error('文档已改变，请重新插入图片');
      const image=await api.InsertImage(owner.id);
      if(!image)return;
      if(active!==owner||content()!==text||currentSelection().some((offset,i)=>offset!==position[i]))
        return notify('文档或光标已改变，请重新插入图片。图片地址：'+image.reference,true);
      return applyTextEdit(position[0],position[1],writeLink(image.label,image.reference,'',true),true);
    }
    case "table": {
      const r = await dialog(
        "插入表格",
        '<label>列数<input name="cols" type="number" min="1" max="20" value="3"></label><label>行数<input name="rows" type="number" min="1" max="100" value="3"></label>',
        [
          ["插入", "ok"],
          ["取消", "cancel"],
        ],
      );
      if (r.value !== "ok") return;
      const cols = Math.max(1, Math.min(20, Number(r.data.cols) || 3)),
        rows = Math.max(1, Math.min(100, Number(r.data.rows) || 3));
      return insert(
        "\n\n| " +
          Array.from({ length: cols }, (_, i) => "列 " + (i + 1)).join(" | ") +
          " |\n| " +
          Array(cols).fill("---").join(" | ") +
          " |\n" +
          Array(rows)
            .fill("| " + Array(cols).fill(" ").join(" | ") + " |")
            .join("\n") +
          "\n\n",
      );
    }
    case "quote":
    case "list":
    case "ordered-list":
    case "check":
    case "indent":
    case "outdent":
    case "line":
      if (source && (id === "indent" || id === "outdent"))
        return editSource((v, a, b) => {
          const [start, end] = lineBounds(v, a, b);
          const lines = v
            .slice(start, end)
            .split("\n")
            .map((l) =>
              id === "indent"
                ? " ".repeat(settings.indent) + l
                : l.replace(new RegExp(`^(?: {1,${settings.indent}}|\\t)`), ""),
            );
          return { start, end, text: lines.join("\n") };
        });
      if (
        !source &&
        !(id === "list" && settings.markdown.listMarker !== "-") &&
        toolbar(id)
      )
        return;
      return insert(
        {
          quote: "> ",
          list: settings.markdown.listMarker + " ",
          "ordered-list": "1. ",
          check: "- [ ] ",
          indent: " ".repeat(settings.indent),
          outdent: "",
          line: "\n\n---\n\n",
        }[id],
      );
    case "codeblock":
      return wrap("\n\n```\n", "\n```\n\n");
    case "mathblock":
      return wrap("\n\n$$\n", "\n$$\n\n");
    case "footnote": {
      const name = await promptText("脚注", "脚注标识", "1");
      if (name !== null) return insert(`[^${name}]\n\n[^${name}]: 脚注内容\n`);
      return;
    }
    case "reference":
      return insert("[链接][ref]\n\n[ref]: https://example.com\n");
    case "toc":
      return insert("\n\n[TOC]\n\n");
    case "yaml":
      return editSource((v) => ({
        start: 0,
        end: 0,
        text: "---\ntitle: " + active.name + "\n---\n\n",
      }));
    case "insertbefore":
    case "insertafter":
      return editSource((v, a, b) => {
        const [start, end] = lineBounds(v, a, b);
        return {
          start: id === "insertbefore" ? start : end,
          end: id === "insertbefore" ? start : end,
          text: "\n\n",
        };
      });
    case "heading-up":
    case "heading-down":
      return editSource((v, a, b) => {
        const [start, end] = lineBounds(v, a, b);
        const text = v.slice(start, end),
          m = /^(#{1,6})\s+/.exec(text);
        const level = Math.max(
          1,
          Math.min(6, (m ? m[1].length : 2) + (id === "heading-up" ? -1 : 1)),
        );
        return {
          start,
          end,
          text: "#".repeat(level) + " " + text.replace(/^#{1,6}\s+/, ""),
        };
      });
    case "task-toggle":
    case "task-complete":
    case "task-incomplete": {
      if(!commandEnabled(id))return;
      const [a,b]=currentSelection(),[start,end]=lineBounds(content(),a,b),before=content().slice(start,end);
      const after=taskStates(before,id.slice(5));
      if(after!==before){applyTextEdit(start,end,after);setMarkdownSelection(a,b);}
      refreshMenuAvailability();
      return;
    }
    case "find":
    case "replace":
      return showFind();
    case "findnext":
      return findNext();
    case "findprev":
      return findNext(true);
    case "final-newline":
      settings.finalNewline = !settings.finalNewline;
      return persist();
    case "paragraph-indent":
      settings.paragraphIndent = !settings.paragraphIndent;
      await persist();
      return mountEditor();
    case "keep-single-line-breaks":
      settings.markdown.keepSingleLineBreaks = settings.markdown.keepSingleLineBreaks === false;
      return persist();
    case "show-br-markers":
      settings.markdown.showBRMarkers = settings.markdown.showBRMarkers === false;
      return persist();
    case "trimspaces":
      return editSource((v) => ({
        start: 0,
        end: v.length,
        text: v.replace(/[ \t]+$/gm, ""),
      }));
    case "spell-language": {
      if(!active)return;
      const owner=active,choice=owner.spellLanguage||'';
      const r=await dialog('文档拼写语言',`<label>当前文档<select name="language">${[['','使用默认设置'],['none','不使用拼写检查'],['en-US','English (US)'],['en-GB','English (UK)']].map(([value,label])=>`<option value="${value}" ${choice===value?'selected':''}>${label}</option>`).join('')}</select></label>`,[['应用','ok'],['取消','cancel']]);
      if(r.value==='ok'){owner.spellLanguage=await api.SetDocumentSpelling(owner.id,r.data.language);if(active===owner){applyEditorSpellcheck();scheduleOfflineSpelling();}}
      return;
    }
    case "spellcheck":
      settings.spellcheck = !settings.spellcheck;
      return persist();
    case "math-refresh":
      return mountEditor();
    case "emoji": {
      const r = await promptText(
        "表情与符号",
        "输入表情，例如 :smile: 或直接输入 Unicode",
        "😀",
      );
      if (r !== null) insert(r);
      return;
    }
    case "customcss": {
      const r = await dialog(
        "自定义 CSS",
        `<textarea name="css" class="css-input">${esc(settings.customCSS || "")}</textarea>`,
        [
          ["应用", "ok"],
          ["取消", "cancel"],
        ],
      );
      if (r.value === "ok") {
        settings.customCSS = r.data.css;
        await persist();
      }
      return;
    }
    case "about":
      return dialog(
        "MarkdownReader",
        "<p>MarkdownReader 0.1.0</p><p>Go + Wails + Vditor</p><p>这是正在对齐 Typora 1.3.8 的阶段版本。完整功能对齐尚未通过验收。</p><p>第三方许可证位于程序目录的 licenses 文件夹。</p>",
      );
    case "help":
      return dialog(
        "使用说明",
        "<p>直接输入 Markdown 即时渲染。Ctrl+/ 切换保留原文的源码编辑。</p><p>Ctrl+O 打开，Ctrl+S 保存，Ctrl+Shift+S 另存为；Ctrl+Shift+L 显示侧边栏。</p><p>F8 专注，F9 打字机，F11 全屏。导出位于文件菜单。</p><p>完整差异与验证记录见随包 docs。</p>",
      );
    default:
      throw Error("未知命令：" + id);
  }
}

async function pasteImageFile(file, target = {owner:active, text:content(), position:currentSelection()}) {
  const {owner, text, position} = target;
  if (active !== owner || content() !== text || currentSelection().some((offset, i) => offset !== position[i])) throw Error('文档或光标已改变，图片粘贴已取消');
  if (!owner.path) throw Error('请先保存文档，再粘贴截图');
  if (!file || file.size > 20 * 1024 * 1024) throw Error('剪贴板图片无效或超过20MB');
  const encoded = await new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(Error('无法读取剪贴板图片'));
    reader.onload = () => resolve(String(reader.result).split(',')[1]);
    reader.readAsDataURL(file);
  });
  if (active !== owner || content() !== text || currentSelection().some((offset, i) => offset !== position[i])) throw Error('文档或光标已改变，图片粘贴已取消');
  await flush();
  if (active !== owner || content() !== text || currentSelection().some((offset, i) => offset !== position[i])) throw Error('文档或光标已改变，图片粘贴已取消');
  const path = await api.ImportClipboardImage(owner.id, encoded);
  if (active !== owner || content() !== text || currentSelection().some((offset, i) => offset !== position[i])) {
    return notify('图片已处理，返回地址为 '+path+'；光标或文档已改变，未插入正文', true);
  }
  applyTextEdit(position[0], position[1], writeLink('截图',path,'',true), true);
}
document.addEventListener('paste', e => {
  if (!ready || !active || !e.target.closest('#work')) return;
  pasteSerial++;
  const item = [...(e.clipboardData?.items || [])].find(item => item.kind === 'file' && item.type.startsWith('image/'));
  if (!item) return;
  e.preventDefault(); e.stopImmediatePropagation();
  safely(() => pasteImageFile(item.getAsFile()));
}, true);

$("#source").addEventListener("input", (e) => {
  if (!e.isComposing && !currentComposition) changed($("#source").value);
});
$("#editor").addEventListener("input", (e) => {
  if (ready && !loading && !source && !e.isComposing && !currentComposition) {
    // Read after Vditor's bubbling input handler has normalised its editable DOM.
    changed(readEditor());
  }
});
document.addEventListener('keydown',event=>{if(event.target.closest?.('#editor'))punctuationView.restore();},true);
document.addEventListener('pointerdown',event=>{if(event.target.closest?.('#editor'))punctuationView.restore();},true);
document.addEventListener('selectionchange',()=>schedulePunctuationView());
document.addEventListener('focusin',()=>schedulePunctuationView());
document.addEventListener('focusout',()=>schedulePunctuationView());
document.addEventListener('input',()=>schedulePunctuationView());
document.addEventListener('beforeinput',event=>{if(event.target.closest?.('#editor'))punctuationView.restore();},true);
document.addEventListener('compositionstart',()=>punctuationView.restore(),true);
document.addEventListener('compositionend',()=>schedulePunctuationView(),true);
document.addEventListener("beforeinput", (e) => {
  if (!ready || !active || !e.target.closest("#work")) return;
  lastInputAt = performance.now();
  if (e.inputType === "historyUndo" || e.inputType === "historyRedo") {
    e.preventDefault();
    safely(() => run(e.inputType === "historyUndo" ? "undo" : "redo"));
    return;
  }
  rememberSelection();
  const before = currentSelection();
  rememberHistoryCursor();
  pendingInput = {documentId: active.id, before, type: e.inputType, mode: source ? "source" : "ir", time: performance.now(), composition: currentComposition};
}, true);
document.addEventListener("compositionstart", (e) => {
  if (!e.target.closest("#work")) return;
  currentComposition = ++compositionCounter;
  clearSearchHighlights();
  if (active) active.historyGroup = null;
}, true);
document.addEventListener("compositionend", (e) => {
  if (!e.target.closest("#work")) return;
  const documentId = active?.id;
  const sourceComposition = !!e.target.closest("#source");
  currentComposition = null;
  queueMicrotask(() => {
    if (!ready || loading || active?.id !== documentId || source !== sourceComposition) return;
    changed(sourceComposition ? $("#source").value : readEditor());
    scheduleSearchHighlights();
    if(!sourceComposition)safely(()=>renderEditorMath());
  });
}, true);
document.addEventListener("pointerdown", () => { if (active) active.historyGroup = null; }, true);
document.addEventListener("beforeinput", (e) => {
  if (
    !ready ||
    e.isComposing ||
    e.inputType !== "insertText" ||
    !e.data ||
    e.data.length !== 1 ||
    !e.target.closest("#work")
  )
    return;
  const char = e.data,
    m = settings.markdown;
  const cursor = source ? $("#source").selectionStart : markdownSelection()[0];
  const previous = content().slice(0, cursor);
  if (!source) {
    const anchor = getSelection()?.anchorNode;
    const element = anchor?.nodeType === Node.ELEMENT_NODE ? anchor : anchor?.parentElement;
    // Vditor's editable root is also a PRE. Only nested literal content is code.
    if (element?.closest('pre:not(.vditor-reset),code,[data-type="code"],[data-type="code-block"],[data-type="math-inline"],[data-type="math-block"],[data-type="html-inline"],[data-type="html-block"]')) return;
  }
  if (!source && !m.smartRender && m.smartQuotes && (char === '"' || char === "'")) {
    e.preventDefault();
    const opening = previous === "" || /[\s([{]$/.test(previous);
    const left = char === '"' ? '“' : '‘', right = char === '"' ? '”' : '’';
    const [start, end] = currentSelection();
    if (settings.pair && start === end && content()[start] === right) {
      setMarkdownSelection(start + 1);
    } else if (settings.pair && (opening || start !== end)) {
      const selected = content().slice(start, end);
      applyTextEdit(start, end, left + selected + right);
      setMarkdownSelection(start + 1, start + 1 + selected.length);
    } else {
      insert(opening ? left : right);
    }
    return;
  }
  if (source && !m.smartRender && m.smartDashes && char === "-" && canConvertSmartDash(content(), cursor, $("#source").selectionEnd)) {
    e.preventDefault();
    $("#source").setRangeText("—", cursor - 2, cursor, "end");
    changed($("#source").value);
    return;
  }
  const pairs = { "(": ")", "[": "]", "{": "}", '"': '"', "'": "'" };
  if (settings.markdownPair)
    Object.assign(pairs, { "*": "*", _: "_", "`": "`" });
  const end = pairs[char];
  if (
    end &&
    ((settings.pair && ["(", "[", "{", '"', "'"].includes(char)) ||
      (settings.markdownPair && ["*", "_", "`"].includes(char))) &&
    source
  ) {
    e.preventDefault();
    const t = $("#source"),
      a = t.selectionStart,
      b = t.selectionEnd;
    const selected = t.value.slice(a, b);
    t.setRangeText(char + selected + end, a, b, "end");
    t.setSelectionRange(a + 1, a + 1 + selected.length);
    changed(t.value);
  }
});
$("#source").addEventListener("keydown", (e) => {
  if (e.key === "Tab") {
    e.preventDefault();
    if (e.shiftKey) {
      safely(() => run("outdent"));
      return;
    }
    const t = e.currentTarget;
    const start = t.selectionStart,
      end = t.selectionEnd;
    t.setRangeText(" ".repeat(settings.indent), start, end, "end");
    changed(t.value);
  }
});
document.addEventListener("selectionchange", () => {
  rememberSelection();
  updateDocumentStatistics();
  updateTableTools();
  document
    .querySelectorAll(".active-block")
    .forEach((x) => x.classList.remove("active-block"));
  const s = getSelection();
  const el =
    s?.anchorNode?.nodeType === 1 ? s.anchorNode : s?.anchorNode?.parentElement;
  const block = el?.closest(
    '#editor [data-block="0"],#editor [data-block="1"],#editor p,#editor h1,#editor h2,#editor h3,#editor li',
  );
  block?.classList.add("active-block");
  if (settings?.typewriter && block) {
    block.scrollIntoView({ block: "center", behavior: "smooth" });
  }
});
document.addEventListener(
  "keydown",
  (e) => {
    if (e.isComposing || !settings) return;
    if (["ArrowLeft","ArrowRight","ArrowUp","ArrowDown","Home","End","PageUp","PageDown","Tab","Enter"].includes(e.key) && active) active.historyGroup = null;
    if (e.key === "Escape") {
      closeMenus();
      $("#findbar").hidden = true;
      clearSearchHighlights();
      return;
    }
    if ($("#modal").open) return;
    const inFindbar = !!e.target.closest("#findbar");
    if (e.key === "Tab" && !e.ctrlKey && !e.altKey && !source && tableContext()) {
      e.preventDefault(); e.stopImmediatePropagation();
      safely(() => navigateTable(e.shiftKey));
      return;
    }
    let key = [
      e.altKey ? "alt" : "",
      e.ctrlKey ? "ctrl" : "",
      e.shiftKey ? "shift" : "",
      {
        Equal: "=",
        Minus: "-",
        Backquote: "`",
        BracketLeft: "[",
        BracketRight: "]",
        Slash: "/",
        Backslash: "\\",
        Comma: ",",
        Period: ".",
      }[e.code] ||
        (/^Digit[0-9]$/.test(e.code) ? e.code.slice(-1) : e.key.toLowerCase()),
    ]
      .filter(Boolean)
      .join("+");
    let id;
    for (const [command, custom] of Object.entries(settings.shortcuts || {})) {
      if (custom.toLowerCase() === key) {
        id = command;
        break;
      }
    }
    id ??= keys.get(key);
    if (inFindbar && !['source', 'findnext', 'findprev'].includes(id)) return;
    if (id?.startsWith("table:") && !tableContext()) return;
    if (id === 'paste' && key === 'ctrl+v' && e.target.closest('#work')) return;
    if (id) {
      e.preventDefault();
      e.stopImmediatePropagation();
      if(!commandEnabled(id))return;
      rememberSelection();
      safely(() => run(id));
    }
  },
  true,
);
$("#sidebar-toggle").onclick = () => safely(() => run("sidebar"));
$("#source-toggle").onclick = () => safely(() => run("source"));
document
  .querySelectorAll("[data-side]")
  .forEach((b) => (b.onclick = () => showSidebar(b.dataset.side)));
$("#find-next").onclick = () => safely(() => findNext());
$("#find-prev").onclick = () => safely(() => findNext(true));
$("#find-close").onclick = () => { $("#findbar").hidden = true; clearSearchHighlights(); };
$("#find-query").oninput = () => findMatches();
$("#source").addEventListener('scroll', () => syncSourceSearchScroll($("#source")));
new ResizeObserver(scheduleSearchHighlights).observe($("#source"));
for (const option of ['#find-case', '#find-regex', '#find-word']) $(option).onchange = () => { findPos = -1; findMatches(); };
$("#find-query").onkeydown = (e) => {
  if (e.key === "Enter" && !e.isComposing) {
    // Moving focus into the editor must not also insert this Enter there.
    e.preventDefault();
    e.stopPropagation();
    safely(() => findNext(e.shiftKey));
  }
};
$("#replace-all").onclick = () => safely(replaceAll);
$("#replace-one").onclick = () =>
  safely(async () => {
    const matches = findMatches();
    const [start, end] = currentSelection();
    const match = matches.find(
      (m) =>
        m.index === start && m[0].length === end - start,
    );
    if (!match) return findNext();
    const value = replacementText(match, $("#replace-query").value, content(), $("#find-regex").checked);
    applyTextEdit(start, end, value, true);
    return findNext();
  });
$("#reload").onclick = () =>
  safely(async () => {
    if (active.dirty) {
      const r = await dialog("重新加载", "重新加载将放弃本窗口未保存的更改。", [
        ["重新加载", "ok"],
        ["取消", "cancel"],
      ]);
      if (r.value !== "ok") return;
    }
    Object.assign(active, await api.ReloadDocument(active.id));
  active.savedContent=active.content;active.savedEOL=active.eol;active.savedEncoding=active.encoding;
    await mountEditor();
    $("#conflict").hidden = true;
    updateStatus();
    renderTabs();
  });
$("#conflict-saveas").onclick = () => safely(() => save(true));
$("#conflict-dismiss").onclick = () => ($("#conflict").hidden = true);
$("#count").onclick = () => {
  const stats = documentStatistics(content(), settings.readingSpeed, settings.markdown.strictMode, active.eol);
  const selected = statisticsSelection?.id === active?.id ? documentStatistics(statisticsSelection.text, settings.readingSpeed, settings.markdown.strictMode) : null;
  dialog(
    "文档统计",
    `<h3>总计</h3><p>${stats.words} 词 · ${stats.characters} 个字符 · ${stats.lines} 行</p><p>预计阅读 ${stats.minutes <= 1 ? '≤ 1' : stats.minutes} 分钟</p>${selected ? `<h3>选中部分</h3><p>${selected.words} 词 · ${selected.characters} 个字符</p>` : ''}`,
  );
};
async function boot() {
  for (let i = 0; i < 100 && !window.go?.desktop?.App; i++)
    await new Promise((r) => setTimeout(r, 50));
  api = window.go?.desktop?.App;
  if (!api) throw Error("Go 桌面桥接未加载");
  window.runtime.EventsOn("close-request", () =>
    safely(async () => {
      await api.TraceLifecycle("frontend-close-request");
      await flush();
      await api.RequestClose();
    }),
  );
  const initial = await api.Bootstrap();
  acceptSettings(initial.settings);
  encodingOptions=await api.Encodings();
  menus[0][1].find(item=>item?.id==="reopen-menu").children=[cmd("自动","reopen:auto"),null,...encodingOptions.map(option=>cmd(option.label,"reopen:"+option.name))];
  menus[0][1].find(item=>item?.id==="saveencoding-menu").children=encodingOptions.map(option=>cmd(option.label,"saveencoding:"+option.name));
  recovery = initial.recovery || [];
  folder = settings.lastFolder || "";
  if (folder) tree = await api.FolderTree(folder);
  buildMenus();
  for (const d of initial.documents || []) docs.push(d);
  if (docs.length) await switchDoc(docs[0]);
  else await addDocument(await api.NewDocument());
  if (recovery.length)
    notify(`检测到 ${recovery.length} 份未保存草稿，可在偏好设置中恢复`);
  if(initial.openErrors?.length)notify(initial.openErrors.join("\n"),true);
  setInterval(() => safely(refreshWindowSettings), 2000);
  window.addEventListener('focus', () => safely(refreshWindowSettings));
  setInterval(
    () =>
      safely(async () => {
        if (active?.path) {
          const result = await api.CheckExternal(active.id);
          if (result) $("#conflict").hidden = false;
        }
        if (folder && side === "tree") {
          const next = await api.FolderTree(folder);
          if (JSON.stringify(next) !== JSON.stringify(tree)) {
            tree = next;
            renderTree();
          }
        }
      }),
    4000,
  );
  window.MarkdownReader = {
    run,
    export: doExport,
    getDocument: () => ({ ...active }),
    getSettings: () => ({ ...settings }),
    getEditorDiagnostics: () => ({ready,loading,source,composing:!!currentComposition,punctuationEnabled:punctuationView.enabled,punctuationNodes:punctuationView.records.size,punctuationRootAttached:!!punctuationView.root?.isConnected}),
  };
}
safely(boot);
new MutationObserver(records => {
  for (const record of records) {
    const element = record.target.nodeType === Node.ELEMENT_NODE ? record.target : record.target.parentElement;
    decorateBreakMarkers(element?.closest('[data-type="html-inline"]'));
    if (record.type === 'childList') for (const node of record.addedNodes) if (node.nodeType === Node.ELEMENT_NODE) decorateBreakMarkers(node);
  }
}).observe($("#editor"), {childList: true, subtree: true, characterData: true});
