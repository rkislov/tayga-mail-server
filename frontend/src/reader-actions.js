function mountEditorDialog(id, className) {
  const old=$(id);if(!old)return null;if(old.tagName==='DIALOG')return old;
  const dialog=document.createElement('dialog');dialog.id=id;dialog.className=className;
  while(old.firstChild)dialog.append(old.firstChild);
 dialog.querySelectorAll(".compose-panel").forEach(panel=>{panel.removeAttribute("role");panel.removeAttribute("aria-modal")});
  old.replaceWith(dialog);document.body.append(dialog);
  if(id==='contact-backdrop'){const form=$('form-contact');const actions=form?.querySelector('.actions');if(actions){actions.classList.add('event-editor-footer');const save=actions.querySelector('[type=submit]');if(save){save.className='btn-spray';save.textContent=mailImageText('Сохранить контакт','Save contact')}const cancel=$('btn-contact-close');if(cancel){cancel.className='btn-secondary';cancel.textContent=mailImageText('Отмена','Cancel')}}}
  dialog.addEventListener('click',event=>{if(event.target===dialog){const bounds=dialog.getBoundingClientRect();if(event.clientX<bounds.left||event.clientX>bounds.right||event.clientY<bounds.top||event.clientY>bounds.bottom)dialog.close()}});
  return dialog;
}
function mountCalendarEditor() {
  const dialog=mountEditorDialog('cal-backdrop','calendar-editor-dialog');if(!dialog)return null;
  if(dialog.dataset.prepared)return dialog;dialog.dataset.prepared='true';
  const form=$('form-cal-event');
  const moveField=(id,host)=>{const field=$(id);if(!field)return;const label=field.previousElementSibling;if(label?.tagName==='LABEL'){label.htmlFor=id;host.append(label)}host.append(field)};
  const times=document.createElement('div');times.className='event-time-grid';
  const summary=$('cal-summary');
  for(const id of ['cal-start','cal-end']) {const group=document.createElement('div');group.className='event-field';moveField(id,group);times.append(group)}
  $('cal-location').after(times);
  $('cal-start').required=true;$('cal-end').required=true;
  const calendarLabel=document.createElement('label');calendarLabel.className='field-label';calendarLabel.htmlFor='event-calendar-choice';calendarLabel.textContent=mailImageText('Календарь','Calendar');
  const calendar=document.createElement('select');calendar.id='event-calendar-choice';calendar.className='field-input';times.after(calendarLabel,calendar);
  const details=document.createElement('details');details.className='event-advanced';const heading=document.createElement('summary');heading.textContent=mailImageText('Вложения','Attachments');details.append(heading);
  const resources=$('cal-resource-select')?.closest('.cal-attendee-row');const resourceLabel=resources?.previousElementSibling;if(resourceLabel)form.insertBefore(resourceLabel,form.querySelector(':scope > .actions'));if(resources)form.insertBefore(resources,form.querySelector(':scope > .actions'));
  const freebusy=$('cal-freebusy')?.closest('.cal-freebusy-block');if(freebusy)form.insertBefore(freebusy,form.querySelector(':scope > .actions'));
  moveField('cal-files',details);if($('cal-file-list'))details.append($('cal-file-list'));
  const actions=form.querySelector(':scope > .actions');actions?.before(details);
  if(actions){actions.classList.add('event-editor-footer');const save=actions.querySelector('[type=submit]');if(save){save.className='btn-spray';save.removeAttribute('data-i18n-title');save.removeAttribute('data-i18n');save.textContent=mailImageText('Сохранить событие','Save event')}const cancel=$('btn-cal-close');if(cancel){cancel.className='btn-secondary';cancel.textContent=mailImageText('Отмена','Cancel')}}
  if(actions){const content=document.createElement('div');content.className='event-editor-content';for(const child of [...form.children])if(child!==actions&&child.id!=='cal-msg')content.append(child);form.prepend(content)}
  const title=dialog.querySelector('h3');if(title){title.id='event-editor-title';dialog.setAttribute('aria-labelledby',title.id)}
  return dialog;
}
function syncEventCalendarChoice() {
  const select=$('event-calendar-choice');if(!select)return;
  select.replaceChildren();for(const calendar of calState.calendars.filter(calendar=>calendar.writable!==false)){const option=document.createElement('option');option.value=calendar.id;option.textContent=calendarDisplayLabel(calendar);select.append(option)}
  select.value=calState.calendarID;
}

async function openEntityModal(kind,id) {
  let dialog=$('entity-reader-dialog');if(!dialog){dialog=document.createElement('dialog');dialog.id='entity-reader-dialog';dialog.className='mail-message-dialog';document.body.append(dialog)}
  dialog.dataset.entity=id;dialog.replaceChildren();
  const close=document.createElement('button');close.type='button';close.className='btn-secondary btn-sm';close.textContent=mailImageText('Закрыть','Close');close.addEventListener('click',()=>dialog.close());
  const content=document.createElement('div');content.textContent=mailImageText('Загрузка…','Loading…');dialog.append(close,content);if(!dialog.open)dialog.showModal();
  try{
    const record=await api((kind==='calendar'?'/api/v1/calendar/events/':'/api/v1/contacts/cards/')+encodeURIComponent(id));
    if(!dialog.open||dialog.dataset.entity!==id)return;
    if(kind==='calendar'){calState.eventID=id;showCalReader(record)}else{contactState.cardID=id;showContactReader(record)}
    const reader=$(kind==='calendar'?'cal-reader':'contact-reader');
    const actions=document.createElement('div');actions.className='reader-actions';
    const addButton=(label,callback)=>{const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';button.textContent=label;button.addEventListener('click',callback);actions.append(button);return button};
    if(kind==='contact'){
      addButton(mailImageText('Написать письмо','Write email'),()=>{dialog.close();showApp('mail');openCompose({to:record.email||''})});
      const edit=addButton(mailImageText('Редактировать','Edit'),()=>{dialog.close();editContactRecord(record)});edit.disabled=!!record.readonly;
    }else{
      addButton(mailImageText('Создать похожее событие','Create similar event'),()=>{dialog.close();startNewCalEvent(calEventDayKey(record));$('cal-summary').value=record.summary||'';$('cal-location').value=record.location||'';$('cal-description').value=record.description||''});
    }
    const remove=addButton(mailImageText('Удалить','Delete'),async()=>{
      dialog.close();if(!await askConfirm(t('delete_confirm'),{danger:true})){dialog.showModal();return}
      try{await api((kind==='calendar'?'/api/v1/calendar/events/':'/api/v1/contacts/cards/')+encodeURIComponent(id),{method:'DELETE'});if(kind==='calendar'){calState.eventID='';await loadCalEvents()}else{contactState.cardID='';await loadContactCards()}}catch(error){content.textContent=error.message;dialog.showModal()}
    });
    if(kind==='contact')remove.disabled=!!record.readonly;
    if(kind==='calendar')remove.disabled=calState.calendars.find(calendar=>calendar.id===record.calendar_id)?.writable===false;
    const body=document.createElement('div');
    for(const element of [...reader.children]) {if(element.classList.contains('reader-actions')||element.id==='cal-attach-upload')continue;const copy=element.cloneNode(true);const attachmentTarget=copy.querySelector('#cal-read-attachments');if(attachmentTarget)attachmentTarget.dataset.entityAttachments='true';copy.removeAttribute('id');copy.querySelectorAll('[id]').forEach(child=>child.removeAttribute('id'));body.append(copy)}
    // Attachment buttons are moved with their handlers, then rendered again in the pane.
    if(kind==='calendar'){const original=$('cal-read-attachments');const clone=body.querySelector('[data-entity-attachments]');if(clone)clone.replaceChildren(...original.childNodes);renderCalReadAttachments(record.attachments||[])}
    content.replaceChildren(actions,body);
  }catch(error){content.textContent=error.message}
}
function editContactRecord(record) {
  contactState.editing=record;contactState.bookID=record.book_id;showContactCompose(true);
  for(const key of ['fn','email','tel','org','note'])if($('contact-'+key))$('contact-'+key).value=record[key]||'';
}
// Delegation survives calendar and contact list refreshes.
document.addEventListener('dblclick',event=>{
  const contact=event.target.closest('#contact-list [data-card]');if(contact){event.preventDefault();openEntityModal('contact',contact.dataset.card);return}
  const calendar=event.target.closest('#app-calendar [data-ev]');if(calendar){event.preventDefault();openEntityModal('calendar',calendar.dataset.ev)}
});
