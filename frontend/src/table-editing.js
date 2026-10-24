function pipes(text) {
  const positions = [];
  for (let i = 0; i < text.length; i++) {
    if (text[i] !== "|") continue;
    let slashes = 0;
    for (let j = i - 1; j >= 0 && text[j] === "\\"; j--) slashes++;
    if (slashes % 2 === 0) positions.push(i);
  }
  return positions;
}

export function splitTableRow(line) {
  const prefix = /^( {0,3}(?:> ?)*)(.*)$/.exec(line);
  if (!prefix) return null;
  const body = prefix[2], boundaries = pipes(body);
  if (!boundaries.length) return null;
  let start = 0, end = body.length;
  if (!body.slice(0, boundaries[0]).trim()) start = boundaries.shift() + 1;
  if (boundaries.length && !body.slice(boundaries.at(-1) + 1).trim()) end = boundaries.pop();
  const spans = [];
  for (const stop of [...boundaries, end]) {
    const raw = body.slice(start, stop);
    spans.push({value: raw.trim(), start: prefix[1].length + start + raw.length - raw.trimStart().length, end: prefix[1].length + stop});
    start = stop + 1;
  }
  return {prefix: prefix[1], cells: spans.map(s => s.value), spans};
}

export function markdownTables(text) {
  const lines = [];
  let offset = 0, fence = null;
  for (const raw of text.split("\n")) {
    const marker = /^(?: {0,3}> ?)* {0,3}(`{3,}|~{3,})(.*)$/.exec(raw);
    const unquoted = raw.replace(/^(?: {0,3}> ?)+/, "");
    const blocked = !!fence || !!marker || /^(?: {4}|\t)/.test(unquoted);
    if (marker) {
      if (!fence) fence = marker[1];
      else if (marker[1][0] === fence[0] && marker[1].length >= fence.length && !marker[2].trim()) fence = null;
    }
    lines.push({raw, start: offset, end: offset + raw.length, blocked});
    offset += raw.length + 1;
  }
  const tables = [];
  for (let i = 0; i < lines.length - 1; i++) {
    if (lines[i].blocked || lines[i + 1].blocked) continue;
    const header = splitTableRow(lines[i].raw), divider = splitTableRow(lines[i + 1].raw);
    if (!header || !divider || header.prefix !== divider.prefix || header.cells.length !== divider.cells.length || !divider.cells.every(c => /^:?-+:?$/.test(c))) continue;
    const rows = [header.cells], rowLines = [lines[i]], parsedRows = [header];
    const columns = header.cells.length;
    let last = i + 1, unsupported = false;
    for (let j = i + 2; j < lines.length && !lines[j].blocked; j++) {
      const row = splitTableRow(lines[j].raw);
      if (!row || row.prefix !== header.prefix) break;
      // GFM ignores extra cells and pads short rows; retain extras to avoid data loss.
      if (row.cells.length > columns) unsupported = true;
      rows.push(Array.from({length: columns}, (_, c) => row.cells[c] || ""));
      rowLines.push(lines[j]); parsedRows.push(row); last = j;
    }
    tables.push({start: lines[i].start, end: lines[last].end, prefix: header.prefix, rows, align: divider.cells, rowLines, parsedRows, dividerLine: lines[i + 1], unsupported});
    i = last;
  }
  return tables;
}

export function tableAt(text, offset) {
  const table = markdownTables(text).find(t => offset >= t.start && offset <= t.end);
  if (!table) return null;
  let row = table.rowLines.findIndex(l => offset >= l.start && offset <= l.end);
  if (row < 0) row = 0;
  const local = offset - table.rowLines[row].start;
  let column = table.parsedRows[row].spans.findIndex(s => local <= s.end);
  if (column < 0) column = table.align.length - 1;
  return {...table, row, column};
}

const width = text => [...text].reduce((sum, c) => sum + (/\p{M}/u.test(c) ? 0 : /[\u1100-\u115f\u2e80-\ua4cf\uac00-\ud7a3\uf900-\ufaff\ufe10-\ufe6f\uff00-\uff60\uffe0-\uffe6\p{Extended_Pictographic}]/u.test(c) ? 2 : 1), 0);
function serialize(table, formatted) {
  const widths = table.align.map((_, c) => Math.max(3, ...table.rows.map(r => width(r[c]))));
  const row = cells => table.prefix + "| " + cells.map((cell, c) => cell + (formatted ? " ".repeat(Math.max(0, widths[c] - width(cell))) : "")).join(" | ") + " |";
  const align = table.align.map((a, c) => {
    const left = a.startsWith(":"), right = a.endsWith(":");
    return (left ? ":" : "") + "-".repeat(Math.max(3, (formatted ? widths[c] : 3) - Number(left) - Number(right))) + (right ? ":" : "");
  });
  return [row(table.rows[0]), table.prefix + "| " + align.join(" | ") + " |", ...table.rows.slice(1).map(row)].join("\n");
}

export function transformTable(context, action, options = {}) {
  if (context.unsupported) throw Error("此表格有超出表头的单元格，请先在源码中整理列数，避免丢失内容");
  const table = {...context, rows: context.rows.map(r => [...r]), align: [...context.align]};
  let {row, column} = context;
  row = Math.max(0, Math.min(table.rows.length - 1, row));
  column = Math.max(0, Math.min(table.align.length - 1, column));
  const blank = () => Array(table.align.length).fill("");
  if (action === "row-above" || action === "row-below") {
    // Keep a header row: inserting above the header creates a new header.
    row += Number(action === "row-below");
    table.rows.splice(row, 0, blank());
  } else if (action === "column-left" || action === "column-right") {
    column += Number(action === "column-right");
    table.rows.forEach(r => r.splice(column, 0, "")); table.align.splice(column, 0, "---");
  } else if (action === "row-up" || action === "row-down") {
    const target = row + (action === "row-up" ? -1 : 1);
    if (target < 0 || target >= table.rows.length) return null;
    [table.rows[row], table.rows[target]] = [table.rows[target], table.rows[row]]; row = target;
  } else if (action === "column-leftward" || action === "column-rightward") {
    const target = column + (action === "column-leftward" ? -1 : 1);
    if (target < 0 || target >= table.align.length) return null;
    for (const r of [...table.rows, table.align]) [r[column], r[target]] = [r[target], r[column]];
    column = target;
  } else if (action === "delete-row") {
    table.rows.splice(row, 1); row = Math.min(row, table.rows.length - 1);
  } else if (action === "delete-column") {
    table.rows.forEach(r => r.splice(column, 1)); table.align.splice(column, 1); column = Math.min(column, table.align.length - 1);
  } else if (action.startsWith("align-")) {
    table.align[column] = {"align-left": ":---", "align-center": ":---:", "align-right": "---:"}[action];
    if (!table.align[column]) throw Error("未知对齐方式");
  } else if (action === "resize") {
    const rows = Number(options.rows), cols = Number(options.cols);
    if (!Number.isInteger(rows) || !Number.isInteger(cols) || rows < 1 || cols < 1 || rows > 100 || cols > 100) throw Error("表格行列数必须为 1–100");
    table.rows = Array.from({length: rows}, (_, r) => Array.from({length: cols}, (_, c) => table.rows[r]?.[c] || ""));
    table.align = Array.from({length: cols}, (_, c) => table.align[c] || "---");
    row = Math.min(row, rows - 1); column = Math.min(column, cols - 1);
  } else if (action !== "format" && action !== "delete-table") throw Error("未知表格操作：" + action);
  if (!table.rows.length || !table.align.length || action === "delete-table") return {start: table.start, end: table.end, text: "", cursor: table.start};
  const text = serialize(table, action === "format");
  const output = markdownTables(text)[0];
  const cursor = table.start + output.rowLines[row].start + output.parsedRows[row].spans[column].start;
  return {start: table.start, end: table.end, text, cursor, row, column};
}
