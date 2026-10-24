export function openDeveloperTools() {
  const view=window.chrome?.webview;
  if(!view?.postMessage) return Promise.reject(Error('当前窗口无法调用 WebView2 开发者工具'));
  return new Promise((resolve,reject)=>{
    const id=crypto.randomUUID();
    const finish=error=>{clearTimeout(timer);view.removeEventListener('message',receive);error?reject(Error('无法打开开发者工具：'+error)):resolve();};
    const receive=event=>{
      let result;try{result=typeof event.data==='string'?JSON.parse(event.data):event.data;}catch{return;}
      if(result?.type==='markdownreader:devtools-result'&&result.id===id)finish(result.error);
    };
    const timer=setTimeout(()=>finish('本地窗口未响应'),8000);
    view.addEventListener('message',receive);
    try{view.postMessage('markdownreader:devtools:'+id);}catch(error){finish(error.message);}
  });
}
