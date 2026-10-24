// Offsets use UTF-16, like textarea selections and DOM Range offsets.
export function lineBounds(text, start, end = start) {
  const a = start === 0 ? 0 : text.lastIndexOf("\n", start - 1) + 1;
  // A selection ending at the next line's start excludes that next line.
  const last = end > start && text[end - 1] === "\n" ? end - 1 : end;
  const newline = text.indexOf("\n", last);
  return [a, newline < 0 ? text.length : newline];
}

export function replacementText(match, replacement, input, regex) {
  if (!regex) return replacement;
  return replacement.replace(/\$(\$|&|`|'|<[^>]+>|\d{1,2})/g, (token, key) => {
    if (key === "$") return "$";
    if (key === "&") return match[0];
    if (key === "`") return input.slice(0, match.index);
    if (key === "'") return input.slice(match.index + match[0].length);
    if (key.startsWith("<"))
      return match.groups ? (match.groups[key.slice(1, -1)] ?? "") : token;
    const number = Number(key);
    if (number > 0 && number < match.length) return match[number] ?? "";
    if (key.length === 2 && Number(key[0]) > 0 && Number(key[0]) < match.length)
      return (match[Number(key[0])] ?? "") + key[1];
    return token;
  });
}

export function replaceMatches(input, matches, replacement, regex) {
  let result = input;
  for (const match of [...matches].reverse()) {
    result = result.slice(0, match.index) +
      replacementText(match, replacement, input, regex) +
      result.slice(match.index + match[0].length);
  }
  return result;
}
export function sentenceBounds(text, start, end = start) {
  const [lineStart, lineEnd] = lineBounds(text, start, end);
  const line = text.slice(lineStart, lineEnd);
  const segments = [...new Intl.Segmenter('zh', {granularity: 'sentence'}).segment(line)];
  const first = segments.findIndex((part, index) => start < lineStart + part.index + part.segment.length || index === segments.length - 1);
  if (first < 0) return [lineStart, lineEnd];
  const lastPosition = Math.max(start, end - 1);
  let last = first;
  while (last + 1 < segments.length && lastPosition >= lineStart + segments[last].index + segments[last].segment.length) last++;
  return [lineStart + segments[first].index,
    lineStart + segments[last].index + segments[last].segment.length];
}

