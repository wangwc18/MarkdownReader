import {readFileSync,writeFileSync,mkdirSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {resolve,dirname} from 'node:path';
import {fileURLToPath} from 'node:url';

// Local compatibility patch for the Lute bundle shipped with Vditor 4.0.0.
// Keep the upstream asset untouched and fail closed on dependency changes.
const root=resolve(dirname(fileURLToPath(import.meta.url)),'..');
const source=resolve(root,'frontend/node_modules/vditor/dist/js/lute/lute.min.js');
const target=resolve(root,'frontend/public/vendor/vditor/dist/js/lute/lute.min.js');
const original=readFileSync(source),sha=data=>createHash('sha256').update(data).digest('hex');
const expected='53b61b051a3dfd9ee5a5010c873233679e139d8e62c49da08b4c2f2c195587cb';
if(sha(original)!==expected)throw Error('Lute upstream SHA256 changed; review compatibility patches before building');
let text=original.toString('utf8');
const patches=[
 ['if(e<g.$length&&!F.IsWhitespace(((e<0||e>=g.$length)?($throwRuntimeError("index out of range"),undefined):g.$array[g.$offset+e]))){return[b,c,d,e];}',
  'if(!f.Context.ParseOption.MarkdownReaderLoose&&e<g.$length&&!F.IsWhitespace(((e<0||e>=g.$length)?($throwRuntimeError("index out of range"),undefined):g.$array[g.$offset+e]))){return[b,c,d,e];}'],
 ['c=$subslice(f.Context.currentLine,f.Context.nextNonspace,((f.Context.nextNonspace+e>>0)+1>>0));',
  'c=$subslice(f.Context.currentLine,f.Context.nextNonspace,f.Context.nextNonspace+e+(e<g.$length&&F.IsWhitespace(g.$array[g.$offset+e])?1:0));'],
 ['AN=function GQ(b,c){var b,c;if(c.blank){if(DN.nil===b.FirstChild){return 1;}c.advanceNextNonspace();}else if(c.indent>=(b.ListData.MarkerOffset+b.ListData.Padding>>0)){c.advanceOffset(b.ListData.MarkerOffset+b.ListData.Padding>>0,true);}else{return 1;}return 0;};',
  'AN=function GQ(b,c){var b,c;var markdownReaderPadding=b.ListData.MarkerOffset+(c.ParseOption.MarkdownReaderLoose?Math.min(2,b.ListData.Padding):b.ListData.Padding);if(c.blank){if(DN.nil===b.FirstChild){return 1;}c.advanceNextNonspace();}else if(c.indent>=markdownReaderPadding){c.advanceOffset(markdownReaderPadding,true);}else{return 1;}return 0;};'],
 ['if(3>b.tokensLen||!d.Context.ParseOption.InlineMath){',
  'if(3>b.tokensLen||!d.Context.ParseOption.InlineMath||(b.pos+1<b.tokensLen&&[9,10,13,32].includes(b.tokens.$array[b.tokens.$offset+b.pos+1]))){'],
 ['if((36===((c<0||c>=b.$length)?($throwRuntimeError("index out of range"),undefined):b.$array[b.$offset+c]))&&0<c&&!((92===',
  'if((36===((c<0||c>=b.$length)?($throwRuntimeError("index out of range"),undefined):b.$array[b.$offset+c]))&&0<c&&![9,13,32].includes(b.$array[b.$offset+c-1])&&!((92==='],
];
for(const [before,after] of patches){if(text.split(before).length!==2)throw Error('Lute patch anchor is not unique');text=text.replace(before,after);}
// A disabled, complete ASCII numeric formula is literal as a whole. Leaving
// its closing dollar in the input lets that dollar steal the following formula.
const numericStart=text.indexOf('if(!d.Context.ParseOption.InlineMathAllowDigitAfterOpenMarker&&b.tokensLen>');
const numericEnd=text.indexOf('q=d.matchInlineMathEnd',numericStart);
if(numericStart<0||numericEnd<0)throw Error('Numeric math patch anchors missing');
let numeric=text.slice(numericStart,numericEnd);
const advance='b.pos=b.pos+(3)>>0;';
if(numeric.split(advance).length!==2||numeric.split('(e+3>>0)').length!==3)throw Error('Numeric math patch shape changed');
numeric=numeric.replace(advance,'var markdownReaderEnd=d.matchInlineMathEnd($subslice(b.tokens,(e+1>>0)));var markdownReaderLiteral=markdownReaderEnd>0;for(var markdownReaderIndex=e+1;markdownReaderLiteral&&markdownReaderIndex<e+1+markdownReaderEnd;markdownReaderIndex++){var markdownReaderByte=b.tokens.$array[b.tokens.$offset+markdownReaderIndex];if(markdownReaderByte<33||markdownReaderByte>126||markdownReaderByte===36)markdownReaderLiteral=false;}b.pos=markdownReaderLiteral?e+markdownReaderEnd+2:e+3;').replaceAll('(e+3>>0)','b.pos');
text=text.slice(0,numericStart)+numeric+text.slice(numericEnd);
text='/* MarkdownReader compatibility patches: math delimiters and Typora parser modes; see docs/lute-patches.md. */\n'+text;
// The fixed GopherJS bundle exposes the wrapped Go instance through this own
// property. Keep a separate option on each parser; never rewrite source text.
text+=`\n;(function(){
 const api=typeof window!=='undefined'?window.Lute:globalThis.Lute,create=api.New;
 let defaultStrict=true;
 api.SetMarkdownReaderDefaultStrictMode=enabled=>{defaultStrict=enabled!==false;};
 api.New=function(...args){
  const parser=create.apply(this,args),options=parser.__internal_object__?.ParseOptions;
  if(!options)throw Error('MarkdownReader parser bridge changed');
  parser.SetMarkdownReaderStrictMode=enabled=>{options.MarkdownReaderLoose=enabled===false;};
  parser.SetMarkdownReaderStrictMode(defaultStrict);return parser;
 };
})();\n`;
mkdirSync(dirname(target),{recursive:true});writeFileSync(target,text);
console.log(JSON.stringify({sourceSHA256:expected,patchedSHA256:sha(text),patches:patches.length+2}));
