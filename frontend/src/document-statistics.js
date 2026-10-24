// Statistics use the Markdown source, including code languages and link targets.
// Ordered-list syntax is excluded, while numbers inside literal code are words.
export function statisticsText(source, strict = true) {
  let fence = null, ticks = 0, blank = true, quoteDepth = 0;
  const containers = [];
  return source.split('\n').map(line => {
    const prefix = /^(?: {0,3}>[ \t]?)+/.exec(line)?.[0] || '';
    const depth = (prefix.match(/>/g) || []).length;
    if (depth !== quoteDepth) { containers.length = 0; quoteDepth = depth; }
    const body = line.slice(prefix.length);
    if (!body.trim()) { blank = true; return line; }
    const leading = /^[ \t]*/.exec(body)[0];
    let indent = 0;
    for (const char of leading) indent += char === '\t' ? 4 - indent % 4 : 1;
    const item = /^([-+*]|\d{1,9}[.)])([ \t]+|$)/.exec(body.slice(leading.length));
    if (!fence && !ticks && (item || blank)) {
      while (containers.length && containers.at(-1) > indent) containers.pop();
    }
    const base = containers.at(-1) || 0;
    const relative = indent - base;
    const marker = relative <= 3 ? /^(`{3,}|~{3,})(.*)$/.exec(body.slice(leading.length)) : null;
    if (fence) {
      if (marker && marker[1][0] === fence[0] && marker[1].length >= fence.length && !marker[2].trim()) fence = null;
      return line;
    }
    if (marker && !ticks) { fence = marker[1]; return line; }
    const literal = ticks || relative >= 4;
    let result = line;
    if (!literal && item) {
      const padding = item[2].length > 4 ? 1 : Math.max(1,item[2].length);
      const width = item[1].length + padding;
      containers.push(indent + (strict ? width : Math.min(2,width)));
      if (/^\d/.test(item[1])) {
        const offset = prefix.length + leading.length;
        result = line.slice(0,offset) + line.slice(offset+item[1].length);
      }
    }
    blank = false;
    // Multiline inline-code spans also protect a line that looks like a list.
    for (let i = 0; i < body.length; i++) {
      if (body[i] === '\\') { i++; continue; }
      if (body[i] !== '`') continue;
      const run = /^`+/.exec(body.slice(i))[0].length;
      if (!ticks) ticks = run; else if (ticks === run) ticks = 0;
      i += run - 1;
    }
    return result;
  }).join('\n');
}

export function documentStatistics(source, readingSpeed = 300, strict = true, eol) {
  const text = statisticsText(source, strict);
  const han = (text.match(/[\u3400-\u9fff]/g) || []).length;
  const words = (text.replace(/[\u3400-\u9fff]/g, ' ').match(/[\p{L}\p{N}]+(?:[-'.’][\p{L}\p{N}]+)*/gu) || []).length;
  // The observed Typora version counts this emoji as two UTF-16 units.
  const symbols = (text.match(/\p{Extended_Pictographic}/gu) || []).reduce((n, value) => n + value.length, 0);
  const count = han + words + symbols;
  const normalised = source.replace(/\r\n/g, '\n');
  const characters = eol ? normalised.length + (eol === 'CRLF' ? (normalised.match(/\n/g) || []).length : 0) : source.length;
  return {words:count, characters, lines:source.split('\n').length,
    minutes:Math.max(1, Math.ceil(count / Math.max(1, Number(readingSpeed) || 300)))};
}
