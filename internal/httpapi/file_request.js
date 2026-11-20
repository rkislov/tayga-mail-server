'use strict';
const form=document.getElementById('upload-form'),picker=document.getElementById('documents'),send=document.getElementById('send'),cancel=document.getElementById('cancel'),status=document.getElementById('status'),progress=document.getElementById('progress'),results=document.getElementById('results');
const expires=document.getElementById('expires');expires.textContent=new Date(expires.dateTime).toLocaleString();
let busy=false,current=null,cancelled=false;
const messages={400:'Не удалось принять файл. Проверьте имя и размер.',401:'Для ссылки требуется пароль. Откройте ссылку заново и введите пароль.',403:'Приём файлов недоступен: ссылка закрыта, срок истёк или папка изменена.',404:'Ссылка больше недоступна.',409:'Файл не принят из-за конфликта. Повторите отправку.',410:'Срок действия ссылки истёк.',413:'Файл превышает 10 ГиБ.',423:'Папка занята другой операцией. Повторите позже.',429:'Сейчас отправляется слишком много файлов. Повторите позже.',507:'У владельца недостаточно свободного места. Свяжитесь с ним.'};
cancel.onclick=()=>{cancelled=true;current?.abort()};
function upload(file){return new Promise((resolve,reject)=>{
 const xhr=new XMLHttpRequest();current=xhr;xhr.open('PUT',location.pathname+'?name='+encodeURIComponent(file.name));xhr.setRequestHeader('Content-Type','application/octet-stream');
 xhr.upload.onprogress=event=>{if(event.lengthComputable){progress.value=Math.round(event.loaded/event.total*100);status.textContent='Отправка: '+file.name+' · '+progress.value+'%'}};
 xhr.onload=()=>{current=null;if(xhr.status===201)resolve();else reject(Error(messages[xhr.status]||'Не удалось принять файл. Если соединение оборвалось, уточните у владельца, был ли документ получен, прежде чем отправлять повторно.'))};
 xhr.onerror=()=>{current=null;reject(Error('Соединение прервалось. Уточните у владельца, был ли файл получен, прежде чем отправлять повторно.'))};
 xhr.onabort=()=>{current=null;reject(Error('Отправка отменена. Если файл уже был передан, уточните результат у владельца.'))};
 xhr.send(file);
})}
form.onsubmit=async event=>{
 event.preventDefault();if(busy||!picker.files.length)return;
 busy=true;cancelled=false;picker.disabled=true;send.disabled=true;cancel.hidden=false;progress.hidden=false;results.replaceChildren();
 let accepted=0;
 try{for(const file of [...picker.files]){
  if(cancelled)break;const row=document.createElement('li');results.append(row);progress.value=0;status.textContent='Отправка: '+file.name;
  try{if(file.size>10*1024**3)throw Error('Файл превышает 10 ГиБ.');await upload(file);accepted++;row.textContent=file.name+' — получен'}catch(error){row.textContent=file.name+' — '+error.message;row.className='error';if(cancelled)break}
 }}finally{busy=false;picker.disabled=false;send.disabled=false;cancel.hidden=true;progress.hidden=true;status.textContent='Принято документов: '+accepted+'.';if(accepted)picker.value=''}
};
