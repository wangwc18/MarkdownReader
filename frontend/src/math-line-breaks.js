// Work on the TeX render input only, never the Markdown or data-math source.
export function mathWithLineBreaks(tex, enabled) {
  if (!enabled || /\\(?:begin\s*\{|displaylines\s*\{)/.test(tex)) return tex;
  for (let i=0;i<tex.length;i++) {
    if (tex[i]==='%') { const end=tex.indexOf('\n',i); if(end<0)break; i=end; continue; }
    if (tex[i]!=='\\') continue;
    if (tex[i+1]==='\\' || /^\\newline(?![A-Za-z])/.test(tex.slice(i))) return '\\displaylines{'+tex+'}';
    // Escaped percent, braces and other characters cannot begin commands.
    i++;
  }
  return tex;
}
