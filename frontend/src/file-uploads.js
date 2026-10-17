/* Durable, account-scoped uploads. A browser reload requires selecting the file
   again; persisted chunk hashes verify its entire already uploaded prefix. */
const fileUploadChunkSize = 8 * 1024 * 1024;
async function fileChunkDigest(blob) {
  const digest=await crypto.subtle.digest('SHA-256',await blob.arrayBuffer());
  return [...new Uint8Array(digest)].map(v=>v.toString(16).padStart(2,'0')).join('');
}
function uploadIdentity(file,path,owner,email) {return JSON.stringify([email,owner,path,file.size,file.lastModified]);}
function loadUploadIDs(){try{return JSON.parse(localStorage.getItem('tayga.file-uploads')||'{}')}catch{return {}}}
function rememberUpload(key,id){try{const all=loadUploadIDs();if(id)all[key]={id,at:Date.now()};else delete all[key];for(const [k,v] of Object.entries(all))if(!v?.at||Date.now()-v.at>86400000)delete all[k];localStorage.setItem('tayga.file-uploads',JSON.stringify(all))}catch{}}
function forgetUpload(id){try{const all=loadUploadIDs();for(const [key,value]of Object.entries(all))if(value?.id===id)delete all[key];localStorage.setItem('tayga.file-uploads',JSON.stringify(all))}catch{}}
async function putFileResumable(file,path,onProgress,existingID='',requestedOwner=filesState.ownerID||'',signal) {
  const email=state.email,owner=requestedOwner,key=uploadIdentity(file,path,owner,email);
  const ensureAccount=()=>{signal?.throwIfAborted();if(state.email!==email||!state.tokens?.access_token)throw new Error(mailImageText('Учётная запись изменилась. Загрузка приостановлена.','Account changed. Upload paused.'))};
  const request=async(url,options={})=>{
    ensureAccount();const res=await fetch(url,{...options,signal,headers:{Authorization:'Bearer '+state.tokens.access_token,'Content-Type':'application/json',...options.headers}});
    let data={};try{data=await res.json()}catch{}if(!res.ok){const e=new Error(data.error||'HTTP '+res.status);e.status=res.status;throw e}return data;
  };
  const base='/api/v1/files/uploads';let id=existingID||loadUploadIDs()[key]?.id,upload,chunks=[];
  if(id){try{const data=await request(base+'/'+encodeURIComponent(id));upload=data.upload;chunks=data.chunks||[]}catch(e){if(existingID||![404,410].includes(e.status))throw e;rememberUpload(key,'');id=''}}
  if(upload){
    if(upload.path!==path||upload.size!==file.size||(owner&&upload.owner_id!==owner))throw new Error(mailImageText('Выберите исходный файл с тем же размером.','Select the original file with the same size.'));
    if(upload.complete){forgetUpload(id);onProgress?.(file.size,file.size);return}
    let offset=0;
    for(const chunk of chunks){ensureAccount();if(await fileChunkDigest(file.slice(offset,offset+chunk.size))!==chunk.sha256)throw new Error(mailImageText('Содержимое файла изменилось. Отмените старую загрузку и начните новую.','The file content changed. Cancel the old upload and start a new one.'));offset+=chunk.size;onProgress?.(offset,file.size)}
    if(offset!==upload.offset)throw new Error(mailImageText('Не удалось проверить загруженную часть файла.','Cannot verify the uploaded file prefix.'));
  }else{
    upload=await request(base,{method:'POST',body:JSON.stringify({path,size:file.size,owner_id:owner})});id=upload.id;
  }
  rememberUpload(key,id);let retries=0;
  while(upload.offset<file.size){
    ensureAccount();const start=upload.offset,part=file.slice(start,Math.min(start+fileUploadChunkSize,file.size));
    try{
      upload=await new Promise((resolve,reject)=>{
        const xhr=new XMLHttpRequest();xhr.open('PATCH',base+'/'+encodeURIComponent(id));xhr.timeout=120000;
        const abort=()=>xhr.abort();signal?.addEventListener('abort',abort,{once:true});xhr.onloadend=()=>signal?.removeEventListener('abort',abort);
        xhr.setRequestHeader('Authorization','Bearer '+state.tokens.access_token);xhr.setRequestHeader('Content-Type','application/octet-stream');xhr.setRequestHeader('Upload-Offset',String(start));
        xhr.upload.onprogress=e=>onProgress?.(start+e.loaded,file.size);
        xhr.onload=()=>{let data={};try{data=JSON.parse(xhr.responseText)}catch{}if(xhr.status>=200&&xhr.status<300)resolve(data);else{const error=new Error(data.error||'HTTP '+xhr.status);error.status=xhr.status;reject(error)}};
        xhr.onerror=xhr.ontimeout=xhr.onabort=()=>reject(new Error(mailImageText('Соединение прервано. Выберите тот же файл, чтобы продолжить.','Connection interrupted. Select the same file to resume.')));xhr.send(part);
      });retries=0;
    }catch(e){
      if(e.status&&e.status!==409&&e.status<500||++retries>3)throw e;
      await new Promise(resolve=>setTimeout(resolve,retries*1000));
      const data=await request(base+'/'+encodeURIComponent(id));upload=data.upload;
      let checked=0;for(const chunk of data.chunks||[]){if(checked>=start&&await fileChunkDigest(file.slice(checked,checked+chunk.size))!==chunk.sha256)throw new Error(mailImageText('Содержимое загрузки изменилось в другом окне.','Upload content changed in another window.'));checked+=chunk.size}
      // This page has already verified the prefix. The only writer holding the
      // session is serialized server-side, and retry always uses its saved offset.
      if(upload.offset<start||upload.offset>start+part.size)throw new Error(mailImageText('Загрузка изменена в другом окне. Выберите файл заново.','Upload changed in another window. Select the file again.'));
    }
    onProgress?.(upload.offset,file.size);
  }
  await request(base+'/'+encodeURIComponent(id)+'/complete',{method:'POST'});forgetUpload(id);onProgress?.(file.size,file.size);
}
async function openPendingFileUploads(){
  const view=fileLibraryDialog(mailImageText('Незавершённые загрузки','Unfinished uploads'));
  const hint=document.createElement('p');hint.className='muted';hint.textContent=mailImageText('Файлы до 10 ГиБ. Для продолжения выберите тот же файл — уже принятая часть будет проверена. Незавершённая загрузка хранится 24 часа.','Files up to 10 GiB. Select the original file to resume; the uploaded prefix will be verified. Unfinished uploads expire after 24 hours.');view.body.append(hint);
  const list=document.createElement('div');view.body.append(list);
  const load=async()=>{try{const data=await api('/api/v1/files/uploads');list.replaceChildren();if(!data.entries?.length){view.status.textContent=mailImageText('Нет незавершённых загрузок.','No unfinished uploads.');return}
    for(const item of data.entries){const row=document.createElement('div');row.className='file-history-row';const label=document.createElement('div');label.textContent=item.path+' · '+fmtBytes(item.offset)+' / '+fmtBytes(item.size);const actions=document.createElement('div');actions.className='row';
      const input=document.createElement('input');input.type='file';input.hidden=true;input.onchange=async()=>{const file=input.files?.[0];if(!file)return;for(const button of actions.querySelectorAll('button'))button.disabled=true;try{await putFileResumable(file,item.path,(done,total)=>{view.status.textContent=fmtBytes(done)+' / '+fmtBytes(total)},item.id,item.owner_id);await refreshFiles();await load()}catch(error){view.status.textContent=libraryMessage(error);for(const button of actions.querySelectorAll('button'))button.disabled=false}input.value=''};
      actions.append(fileAction(mailImageText('Продолжить','Resume'),()=>input.click()),fileAction(mailImageText('Отменить загрузку','Cancel upload'),async()=>{if(!await askConfirm(mailImageText('Удалить незавершённую загрузку? Исходный файл останется без изменений.','Delete the unfinished upload? The original file will remain unchanged.')))return;try{await api('/api/v1/files/uploads/'+item.id,{method:'DELETE'});forgetUpload(item.id);await load()}catch(error){view.status.textContent=libraryMessage(error)}}),input);row.append(label,actions);list.append(row)}
  }catch(error){view.status.textContent=libraryMessage(error)}};await load();
}
