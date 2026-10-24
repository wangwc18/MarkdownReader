export const imageInsertPresets = [
  ['none','无特殊操作'],
  ['copy-current','复制图片到当前文件夹（./）'],
  ['copy-assets','复制图片到 ./assets 文件夹'],
  ['copy-filename','复制图片到 ./${filename}.assets 文件夹'],
  ['upload','上传图片'],
  ['copy','复制到指定路径'],
];
export function selectedImageInsertPreset(settings) {
  const action=settings.imageInsertAction||(settings.copyImages?'copy':'none');
  if(action!=='copy')return action;
  if(settings.imageFolderCustom)return 'copy';
  const folder=(settings.imageFolder||'assets').replaceAll('\\','/').replace(/\/+$/,'');
  if(folder==='.'||folder==='')return 'copy-current';
  if(folder==='assets'||folder==='./assets')return 'copy-assets';
  if(folder==='${filename}.assets'||folder==='./${filename}.assets')return 'copy-filename';
  return 'copy';
}
export function imageInsertPresetSettings(preset) {
  const folders={'copy-current':'./','copy-assets':'./assets','copy-filename':'./${filename}.assets'};
  if(preset in folders)return {imageInsertAction:'copy',imageFolder:folders[preset],imageFolderCustom:false};
  if(preset==='copy')return {imageInsertAction:'copy',imageFolderCustom:true};
  return {imageInsertAction:preset};
}
