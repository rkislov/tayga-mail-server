// Account preferences are cached by access token, never shared between logins.
let mailImagePreferences = {mode: "block", senders: []};
let mailImagePreferenceToken = "";
let mailImagePreferencePromise;
async function refreshMailImagePreferences(force = false) {
  const token = state.tokens?.access_token || "";
  if (!token) {mailImagePreferences = {mode: "block", senders: []}; mailImagePreferenceToken = ""; return;}
  if (!force && token === mailImagePreferenceToken && mailImagePreferencePromise) return mailImagePreferencePromise;
  mailImagePreferenceToken = token;
  mailImagePreferences = {mode: "block", senders: []};
  mailImagePreferencePromise = api("/api/v1/mail/image-preferences").then(value => {
    if (state.tokens?.access_token === token) {mailImagePreferences = value; renderMailImageSettings();}
  }).catch(() => { if (mailImagePreferenceToken === token) mailImagePreferencePromise = null; });
  return mailImagePreferencePromise;
}
async function saveMailImagePreferences(value) {
  const saved = await api("/api/v1/mail/image-preferences", {method: "PUT", body: JSON.stringify(value)});
  mailImagePreferences = saved;
  renderMailImageSettings();
}
function mailImageText(ru, en) {return lang === "en" ? en : ru;}
function renderMailImageSettings() {
  let card = $("mail-image-settings");
  if (!card) {
    const host = $("profile-security") || $("app-security");
    if (!host) return;
    card = document.createElement("section"); card.id = "mail-image-settings"; card.className = "card stack-gap"; host.append(card);
  }
  card.replaceChildren();
  const title = document.createElement("h3");title.className = "heading"; title.textContent = mailImageText("Безопасность: внешние изображения", "Security: external images");
  const hint = document.createElement("p"); hint.className = "meta"; hint.textContent = mailImageText("Внешние изображения могут сообщить отправителю об открытии письма. Встроенные изображения показываются всегда.", "External images may tell the sender you opened the email. Embedded images are always displayed.");
  const label = document.createElement("label");label.textContent = mailImageText("Загрузка изображений", "Image loading");
  const select = document.createElement("select");select.className = "field-input";select.id = "mail-image-mode";
  for (const [value, text] of [["block",mailImageText("Спрашивать перед загрузкой", "Ask before loading")],["always",mailImageText("Всегда загружать", "Always load")]]) {const option = document.createElement("option");option.value = value;option.textContent = text;select.append(option);}
  select.value = mailImagePreferences.mode;label.append(select);
  const message = document.createElement("p");message.className = "meta"; message.setAttribute("role", "status");
  select.addEventListener("change", async () => {select.disabled = true;try {await saveMailImagePreferences({...mailImagePreferences, mode:select.value});}catch(error) {select.value = mailImagePreferences.mode;message.textContent = error.message;}finally {select.disabled = false;}});
  const list = document.createElement("div");list.className = "stack-gap";
  for (const sender of mailImagePreferences.senders || []) {
    const row = document.createElement("div");row.className = "mail-image-sender";
    const name = document.createElement("span");name.textContent = sender;
    const remove = document.createElement("button");remove.type = "button";remove.className = "btn-secondary btn-sm";remove.textContent = mailImageText("Убрать исключение", "Remove exception");
    remove.addEventListener("click", async () => {remove.disabled = true;try {await saveMailImagePreferences({...mailImagePreferences, senders:mailImagePreferences.senders.filter(value => value !== sender)});}catch(error) {message.textContent = error.message;remove.disabled = false;}});
    row.append(name,remove);list.append(row);
  }
  const caption = document.createElement("p");caption.className = "meta";caption.textContent = mailImageText("Разрешённые отправители", "Allowed senders");
  card.append(title,hint,label,caption,list,message);
}

let mailPageData;
function renderMailPagination(data, busy = false) {
  let bar = $("mail-pagination");
  if (!bar) {
    const list = $("mail-msg-list"); if (!list) return;
    bar = document.createElement("div");bar.id = "mail-pagination";bar.className = "mail-pagination";
    list.parentElement.insertBefore(bar, list);
  }
  if (data) mailPageData = data;
  const previousData = data || mailPageData;
  const size = mailState.pageSize, offset = mailState.offset;
  const total = previousData?.total;
  const count = previousData?.messages?.length || 0;
  bar.replaceChildren();
  const previous = document.createElement("button");previous.type = "button";previous.className = "btn-secondary btn-sm";previous.textContent = "‹";previous.title = mailImageText("Более новые письма", "Newer messages");previous.setAttribute("aria-label", previous.title);previous.disabled = busy || offset === 0;
  const next = document.createElement("button");next.type = "button";next.className = "btn-secondary btn-sm";next.textContent = "›";next.title = mailImageText("Более старые письма", "Older messages");next.setAttribute("aria-label",next.title);next.disabled = busy || !previousData || (typeof total === "number" ? offset + count >= total : !previousData.has_more);
  const label = document.createElement("span");label.className = "meta";label.setAttribute("role", "status");label.textContent = busy ? mailImageText("Загрузка…", "Loading…") : count ? `${offset + 1}–${offset + count}` + (typeof total === "number" ? mailImageText(` из ${total}`, ` of ${total}`) : "") : "0";
  const select = document.createElement("select");select.className = "field-input";select.setAttribute("aria-label",mailImageText("Писем на странице", "Messages per page"));
  for (const number of [25,50,100]) {const option = document.createElement("option");option.value = number;option.textContent = number + mailImageText(" на странице", " per page");select.append(option);}select.value = size;select.disabled = busy;
  const change = offset => {mailState.offset = offset;mailState.messageID = "";showMailReader(null);refreshMailMessages();};
  previous.addEventListener("click", () => change(Math.max(0,offset-size)));
  next.addEventListener("click", () => change(offset+size));
  select.addEventListener("change", () => {mailState.pageSize = Number(select.value);change(0);});
  bar.append(previous,label,next,select);
  const filters=$("mail-quick-filters");
  if(filters){let toolbar=$("mail-toolbar");if(!toolbar){toolbar=document.createElement("div");toolbar.id="mail-toolbar";bar.before(toolbar)}toolbar.append(filters,bar)}
}

async function openMailModal(id) {
  let dialog = $("mail-message-dialog");
  if (!dialog) {
    dialog = document.createElement("dialog");dialog.id = "mail-message-dialog";dialog.className = "mail-message-dialog";
    const close = document.createElement("button");close.type = "button";close.className = "btn-secondary btn-sm";close.textContent = mailImageText("Закрыть", "Close");close.addEventListener("click",()=>dialog.close());
    const content = document.createElement("div");content.id = "mail-modal-content";
    dialog.append(close, content);document.body.append(dialog);
  }
  const content = $("mail-modal-content");content.textContent = mailImageText("Загрузка…", "Loading…");
  dialog.dataset.message = id;if (!dialog.open) dialog.showModal();
  try {
    const msg = await api("/api/v1/mail/messages/"+encodeURIComponent(id));await refreshMailImagePreferences();
    if (!dialog.open || dialog.dataset.message !== id) return;
    // Clone the rendered reader so attachment handlers and image choices retain their behavior.
    showMailReader(msg);
    const reader = $("mail-reader");
    content.replaceChildren();
    const subject = document.createElement("h2");subject.className = "heading reader-subject";subject.textContent = msg.subject;
    const headers = reader.querySelector(".reader-headers")?.cloneNode(true);
    headers?.querySelectorAll("[id]").forEach(element => element.removeAttribute("id"));
    const body = document.createElement("div");body.className = "reader-body";
    body.append(...$("mail-body").childNodes);
    content.append(createMailActionBar(msg,dialog),subject);if(headers)content.append(headers);content.append(body);
    mailState.messageID = id;
    // Restore the pane without cloning the interactive attachment buttons.
    showMailReader(msg);
  } catch (error) {content.textContent = error.message;}
}

const directorySorts = new Map();
function sortDirectoryRows(list, column, direction) {
  const rows = [...list.children].filter(row => row.querySelector('[data-name],[data-card]') || row.tagName === 'TR');
  const selectors = list.id === 'files-list' ? ['.file-name','.file-size','.file-kind'] : list.id === 'contact-list' ? ['.msg-from','.msg-subject','.msg-date'] : null;
  const value = row => {
    if (list.id === 'files-list' && column === 1) {const entry = filesState.entries.find(entry => entry.name === row.querySelector('[data-name]')?.dataset.name);return entry?.size || 0;}
    const cell = selectors ? row.querySelector(selectors[column]) : row.children[column];
    return (cell?.querySelector('select')?.selectedOptions[0]?.textContent || cell?.textContent || '').trim();
  };
  const sorted = rows.slice().sort((a,b) => {
    const x=value(a),y=value(b);const result=typeof x==='number'&&typeof y==='number'?x-y:String(x).localeCompare(String(y),lang,{numeric:true,sensitivity:'base'});
    return direction === 'asc' ? result : -result;
  });
  if (sorted.some((row,index)=>row!==rows[index])) sorted.forEach(row => list.append(row));
}
function mountSortableHeaders() {
  document.querySelectorAll('.msg-list-head, table thead tr').forEach(header => {
    const mail = header.closest('#app-mail');
    const list = mail ? $('mail-msg-list') : header.classList.contains('files-list-head') ? $('files-list') : header.classList.contains('contact-list-head') ? $('contact-list') : header.closest('table')?.querySelector('tbody');
    if (!list) return;
    header.removeAttribute('aria-hidden');
    [...header.children].forEach((cell,column) => {
      if (cell.dataset.sortMounted || cell.dataset.i18n?.includes('actions')) return;
      cell.dataset.sortMounted='true';cell.classList.add('sortable-header');cell.tabIndex=0;cell.setAttribute('role','button');
      const sort=()=>{
        let direction;
        if(mail){const field=['from','subject','date'][column];if(!field)return;direction=mailState.sort===field&&mailState.sortOrder==='asc'?'desc':'asc';mailState.sort=field;mailState.sortOrder=direction;mailState.offset=0;mailState.messageID='';refreshMailMessages();}
        else {const previous=directorySorts.get(list);direction=previous?.column===column&&previous.direction==='asc'?'desc':'asc';directorySorts.set(list,{column,direction});sortDirectoryRows(list,column,direction);}
        [...header.children].forEach(item=>{delete item.dataset.sortDirection;item.removeAttribute('aria-sort')});cell.dataset.sortDirection=direction;cell.setAttribute('aria-sort',direction==='asc'?'ascending':'descending');
      };
      cell.addEventListener('click',sort);cell.addEventListener('keydown',event=>{if(event.key==='Enter'||event.key===' '){event.preventDefault();sort()}});
    });
    if(mail){[...header.children].forEach((cell,column)=>{const field=['from','subject','date'][column];if(field===mailState.sort){cell.dataset.sortDirection=mailState.sortOrder;cell.setAttribute('aria-sort',mailState.sortOrder==='asc'?'ascending':'descending')}})}
  });
  for(const [list,sort] of directorySorts) if(list.isConnected) sortDirectoryRows(list,sort.column,sort.direction);
}
let sortMountQueued=false;
new MutationObserver(()=>{if(!sortMountQueued){sortMountQueued=true;queueMicrotask(()=>{sortMountQueued=false;mountSortableHeaders()})}}).observe(document.body,{childList:true,subtree:true});
mountSortableHeaders();

function ensureComposeCC() {
  if ($('compose-cc') || !$('compose-to')) return;
  const label=document.createElement('label');label.className='field-label';label.htmlFor='compose-cc';label.textContent=mailImageText('Копия','Cc');
  const input=document.createElement('input');input.id='compose-cc';input.className='field-input';
  $('compose-to').after(label,input);
}
function replyToMessage(message, all=false, dialog=null) {
  if (dialog?.open) dialog.close();
  const direct=message.reply_to || message.from || '';
  const email=(direct.match(/<([^>]+)>/) || [])[1] || direct.trim();
  const to=all?(message.reply_all_to || []).join(', '):email;
  let quoted=message.text || '';
  if (!quoted && message.html) {const doc=new DOMParser().parseFromString(message.html,'text/html');doc.querySelectorAll('script,style').forEach(node=>node.remove());quoted=doc.body.textContent || '';}
  ensureComposeCC();
  openCompose({to,cc:all?(message.reply_all_cc || []).join(', '):'',subject:/^re\s*:/i.test(message.subject || '')?message.subject:'Re: '+(message.subject || ''),body:'\n\n---\n'+quoted.slice(0,10000)});
}
function createMailActionBar(message, dialog=null) {
  const bar=document.createElement('div');bar.className='reader-actions';
  for (const [action,label] of [['trip',mailImageText('Поездка в календарь','Trip to calendar')],['reply',mailImageText('Ответить','Reply')],['reply-all',mailImageText('Ответить всем','Reply all')],['spam',mailImageText('В спам','Mark as spam')],['archive',mailImageText('В архив','Archive')],['delete',mailImageText('Удалить','Delete')]]) {
    const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';button.title=label;button.setAttribute('aria-label',label);button.classList.add('action-icon');button.innerHTML=designIcon(action);button.dataset.mailAction=action;if(!dialog)button.id='btn-mail-'+action;
    if(action==='spam' && (mailState.mailboxes || []).some(folder=>folder.id===message.mailbox_id&&folder.name==='Junk'))button.disabled=true;
    button.addEventListener('click',async()=>{
      if(action==='trip'){if(dialog?.open)dialog.close();await openTravelDrafts(message.id);return;}
      if(action==='reply'||action==='reply-all'){replyToMessage(message,action==='reply-all',dialog);return;}
      if(action==='delete'){if(dialog?.open)dialog.close();if(!await askConfirm(t('delete_confirm'),{danger:true})){if(dialog)dialog.showModal();return;}}
      button.disabled=true;
      try {
        if(action==='spam') {
          let folders=mailState.mailboxes || [];
          if(!folders.some(folder=>folder.name==='Junk'))folders=(await api('/api/v1/mail/mailboxes')).mailboxes || [];
          const folder=folders.find(folder=>folder.name==='Junk');if(!folder)throw new Error(mailImageText('Папка спама недоступна','Spam folder unavailable'));
          await api('/api/v1/mail/messages/'+encodeURIComponent(message.id)+'/move',{method:'POST',body:JSON.stringify({mailbox_id:folder.id})});
        } else await api('/api/v1/mail/messages/'+encodeURIComponent(message.id)+(action==='archive'?'/archive':''),{method:action==='archive'?'POST':'DELETE'});
        if(dialog?.open)dialog.close();if(mailState.messageID===message.id)mailState.messageID='';await refreshMail();
      }catch(error){button.disabled=false;if(dialog){const message=document.createElement("p");message.className="msg";message.setAttribute("role","alert");message.textContent=error.message;bar.append(message)}else await askAlert(error.message)}
    });bar.append(button);
  }
  return bar;
}
function renderMailActions(message) {
  const reader=$('mail-reader');if(!reader)return;
  const old=reader.querySelector('.reader-actions');const bar=createMailActionBar(message);
  const back=old?.querySelector('[data-pane-back]');if(back)bar.prepend(back);
  if(old)old.replaceWith(bar);else reader.prepend(bar);
}
function renderMailQuickFilters() {
  let bar=$('mail-quick-filters');if(!bar){const list=$('mail-msg-list');if(!list)return;bar=document.createElement('div');bar.id='mail-quick-filters';bar.className='mail-quick-filters';bar.setAttribute('aria-label',mailImageText('Быстрые фильтры','Quick filters'));list.parentElement.insertBefore(bar,list);}
  bar.replaceChildren();
  for(const [value,label] of [['all',mailImageText('Все','All')],['unread',mailImageText('Непрочитанные','Unread')],['flagged',mailImageText('Отмеченные','Flagged')]]){
    const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';button.textContent=label;button.setAttribute('aria-pressed',String(mailState.filter===value));
    button.addEventListener('click',()=>{mailState.filter=value;mailState.offset=0;mailState.messageID='';refreshMailMessages()});bar.append(button);
  }
}
