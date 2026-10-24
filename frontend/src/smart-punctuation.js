// Only convert prose. Source syntax must survive while it is being typed,
// including incomplete code fences, code spans, HTML tags and link targets.
export function canConvertSmartDash(text, start, end = start) {
  if (start !== end || !text.slice(0, start).endsWith('--')) return false;
  const prefix = text.slice(0, start);
  const lines = prefix.split('\n');
  const unwrap = line => line.replace(/^(?: {0,3}> ?)+/, '')
    .replace(/^ {0,3}(?:[-+*]|\d{1,9}[.)]) +/, '');
  const line = unwrap(lines.at(-1));
  let slash = start - 3, escapes = 0;
  while (text[slash--] === '\\') escapes++;
  if (escapes % 2 || /(?:https?:\/\/|mailto:|file:\/\/)[^\s<>]*$/i.test(prefix) ||
      /^ {0,3}\[[^\]]+\]:\s*\S/.test(line)) return false;
  // Thematic rules, front matter and table separator rows are Markdown syntax.
  if (/^[\s:|*-]*$/.test(line)) return false;
  if (/^(?: {4}|\t)/.test(lines.at(-1).replace(/^(?: {0,3}> ?)+/, ''))) return false;
  if (lines[0].replace(/^\uFEFF/, '') === '---' &&
      !lines.slice(1).some(value => /^(?:---|\.\.\.)\s*$/.test(value))) return false;
  let fence = null, mathBlock = false;
  for (const value of lines) {
    const body = unwrap(value);
    const marker = /^ {0,3}(`{3,}|~{3,})(.*)$/.exec(body);
    if (marker) {
      if (!fence) fence = marker[1];
      else if (marker[1][0] === fence[0] && marker[1].length >= fence.length && !marker[2].trim()) fence = null;
    }
    if (!fence && /^ {0,3}\$\$\s*$/.test(body)) mathBlock = !mathBlock;
  }
  if (fence || mathBlock) return false;
  let ticks = 0, tag = false, quote = '', literalTag = '', destination = 0, math = false;
  for (let i = 0; i < prefix.length; i++) {
    const char = prefix[i];
    if (char === '\\') { i++; continue; }
    if (literalTag) {
      const closing = new RegExp(`^<\\/${literalTag}\\s*>`, 'i').exec(prefix.slice(i));
      if (closing) { literalTag = ''; i += closing[0].length - 1; }
      continue;
    }
    if (tag) {
      if (quote) { if (char === quote) quote = ''; }
      else if (char === '"' || char === "'") quote = char;
      else if (char === '>') tag = false;
      continue;
    }
    if (char === '`') {
      const run = /^`+/.exec(prefix.slice(i))[0].length;
      if (!ticks) ticks = run; else if (ticks === run) ticks = 0;
      i += run - 1; continue;
    }
    if (ticks) continue;
    if (char === '<' && /[A-Za-z/!]/.test(prefix[i + 1] || '')) {
      const literal = /^<(pre|code|script|style)\b[^>]*>/i.exec(prefix.slice(i));
      if (literal) { literalTag = literal[1]; i += literal[0].length - 1; }
      else tag = true;
      continue;
    }
    if (prefix.startsWith('](', i)) { destination++; i++; continue; }
    if (destination) {
      if (char === '(') destination++;
      if (char === ')') destination--;
      continue;
    }
    if (char === '$') math = !math;
    if (char === '\n' && prefix[i + 1] === '\n') { ticks = 0; math = false; }
  }
  return !ticks && !tag && !literalTag && !destination && !math;
}
