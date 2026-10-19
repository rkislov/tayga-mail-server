/* Upload on selection; Send only submits immutable file references. Files stay
   in the user's mail-out folder, including after a draft is closed. */
const composeAttachments = {entries:[], epoch:0, sending:false, queue:Promise.resolve(), preparing:null, limits:null};
function resetComposeAttachments() {
  composeAttachments.epoch++;
  for(const item of composeAttachments.entries)item.controller?.abort();
  composeAttachments.entries=[];composeAttachments.queue=Promise.resolve();composeAttachments.preparing=null;composeAttachments.limits=null;
  if($('compose-attach'))$('compose-attach').value='';
  renderComposeAttachments();
}
function renderComposeAttachments() {
  const items=composeAttachments.entries, busy=composeAttachments.sending||!!composeAttachments.preparing;
  const send=$('btn-compose-send');if(send)send.disabled=busy||items.some(item=>item.status!=='ready');
  if($('compose-attach'))$('compose-attach').disabled=composeAttachments.sending;
  const list=$('compose-uploads');if(!list)return;list.replaceChildren();list.hidden=!items.length;
  for(const item of items){
    const row=document.createElement('div');row.className='compose-upload';row.dataset.uploadId=item.id;
    const content=document.createElement('div'),name=document.createElement('strong'),status=document.createElement('small');name.textContent=item.file.name;
    status.textContent=item.error||({queued:mailImageText('Ожидает загрузки','Waiting to upload'),uploading:item.forward?mailImageText('Подготовка вложения на сервере…','Preparing attachment on server…'):fmtBytes(item.done)+' / '+fmtBytes(item.file.size),ready:item.share?mailImageText('Ссылка в письме · действует до ','Link in message · expires ')+new Date(item.share.expires_at).toLocaleDateString():mailImageText('Загружено · вложение','Uploaded · attachment')}[item.status]||'');
    content.append(name,status);
    if(['queued','uploading'].includes(item.status)){
      const progress=document.createElement('progress');progress.max=item.file.size||1;progress.value=item.done;progress.setAttribute('aria-label',mailImageText('Загрузка ','Uploading ')+item.file.name);content.append(progress);
      if(item.forward)progress.removeAttribute('value');
    }
    const actions=document.createElement('div');actions.className='compose-upload-actions';
    if(item.status==='error'){const retry=document.createElement('button');retry.type='button';retry.className='btn-secondary btn-sm';retry.textContent=mailImageText('Повторить','Retry');retry.disabled=busy;retry.onclick=()=>queueComposeUpload(item);actions.append(retry)}
    const remove=document.createElement('button');remove.type='button';remove.className='btn-secondary btn-sm';remove.textContent='×';remove.title=mailImageText('Убрать из письма','Remove from message');remove.setAttribute('aria-label',remove.title+' '+item.file.name);remove.disabled=busy;
    remove.onclick=()=>{item.controller?.abort();composeAttachments.entries=composeAttachments.entries.filter(value=>value!==item);composeBodyEl()?.querySelector('[data-upload-link="'+item.id+'"]')?.remove();renderComposeAttachments()};actions.append(remove);row.append(content,actions);list.append(row);
  }
}
function addComposeFileLink(item){
  const body=composeBodyEl();if(body.querySelector('[data-upload-link="'+item.id+'"]'))return;
  const paragraph=document.createElement('p');paragraph.dataset.uploadLink=item.id;
  const link=document.createElement('a');link.href=item.share.url;link.textContent=item.file.name;paragraph.append(link,document.createTextNode(' ('+fmtBytes(item.file.size)+') — '+mailImageText('ссылка до ','link expires ')+new Date(item.share.expires_at).toLocaleDateString()));body.append(paragraph);
}
async function prepareComposeAttachments() {
  if(composeAttachments.preparing)return composeAttachments.preparing;
  const epoch=composeAttachments.epoch;
  const work=(async()=>{
    if(!composeAttachments.entries.length)return;
    const fetchedLimits=composeAttachments.limits||await api('/api/v1/mail/limits');
    if(epoch!==composeAttachments.epoch)throw new Error('Draft changed');
    composeAttachments.limits=fetchedLimits;
    const limits=composeAttachments.limits;
    // Reserve headers/boundaries and base64 + CRLF overhead for both body forms.
    const bodyBytes=new TextEncoder().encode(getComposeHTML()+getComposeText()+($('compose-subject')?.value||'')).length;
    let remaining=Math.max(0,limits.message_bytes-Math.ceil(bodyBytes/57)*78-65536),total=0;
    for(const item of composeAttachments.entries){
      if(item.status!=='ready')continue;
      const encoded=Math.ceil(item.file.size/57)*78+2048;
      if(!item.share && (item.file.size>=limits.large_attachment_bytes||total+item.file.size>limits.attachment_bytes||encoded>remaining)){
        const share=await api('/api/v1/files/shares',{method:'POST',body:JSON.stringify({path:item.path,ttl_hours:limits.share_ttl_hours})});
        if(epoch!==composeAttachments.epoch)throw new Error('Draft changed');
        item.share=share;
      }
      if(item.share)addComposeFileLink(item);else{remaining-=encoded;total+=item.file.size}
    }
  })();
  composeAttachments.preparing=work;renderComposeAttachments();
  try{await work}finally{if(composeAttachments.preparing===work){composeAttachments.preparing=null;renderComposeAttachments()}}
}
function queueComposeUpload(item){
  const epoch=composeAttachments.epoch,account=state.me?.id;item.status='queued';item.error='';renderComposeAttachments();
  composeAttachments.queue=composeAttachments.queue.catch(()=>{}).then(async()=>{
    const current=()=>epoch===composeAttachments.epoch&&account===state.me?.id&&composeAttachments.entries.includes(item);
    if(!current())return;
    item.status='uploading';item.controller=new AbortController();renderComposeAttachments();
    try{
      if(!item.ref){
        if(item.forward){
          const ref=await api('/api/v1/mail/messages/'+encodeURIComponent(item.forward.message)+'/attachments/'+encodeURIComponent(item.forward.attachment)+'/prepare',{method:'POST',signal:item.controller.signal});
          if(!current())return;
          item.ref=ref;item.path=ref.path;item.file={name:ref.name,size:ref.size};
        }else{
        await putFileResumable(item.file,item.path,(done)=>{if(current()){item.done=done;const progress=$('compose-uploads')?.querySelector('[data-upload-id="'+item.id+'"] progress');if(progress)progress.value=done;const label=progress?.previousElementSibling;if(label)label.textContent=fmtBytes(done)+' / '+fmtBytes(item.file.size)}},'','',item.controller.signal);
        if(!current())return;
        item.ref=await api('/api/v1/mail/attachments',{method:'POST',body:JSON.stringify({path:item.path,name:item.file.name})});
        }
      }
      if(!current())return;item.status='ready';
      await prepareComposeAttachments();
    }catch(error){if(current()){item.status='error';item.error=error.message}}
    finally{if(current())renderComposeAttachments()}
  });
}
$('compose-attach')?.addEventListener('change',event=>{
  for(const file of Array.from(event.target.files||[])){
    if(composeAttachments.entries.length>=20){setMsg($('compose-msg'),mailImageText('Не больше 20 файлов в одном письме.','Up to 20 files per message.'),'err');break}
    if(file.size>WEB_UPLOAD_BYTES){setMsg($('compose-msg'),mailImageText('Максимальный размер файла — 10 ГиБ.','Maximum file size is 10 GiB.'),'err');continue}
    const id=crypto.randomUUID();let filename=file.name.replace(/[\\/\x00-\x1f\x7f]/g,'_')||'file';while(new TextEncoder().encode(filename).length>180)filename=filename.slice(0,-1);
    const item={id,file,path:'mail-out/'+id+'/'+filename,status:'queued',done:0};composeAttachments.entries.push(item);queueComposeUpload(item);
  }
  event.target.value='';
});
