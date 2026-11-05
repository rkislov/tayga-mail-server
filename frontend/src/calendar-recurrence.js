// Recurrence controls share one contract for creation and whole-series edits.
function createRecurrenceFields(prefix, info = {}, timezone = '', start = '') {
  const host=document.createElement('fieldset');host.className='cal-recurrence-fields';host.id=prefix;
  const rule=info.rule || {frequency:'',interval:1};
  const field=(key,label,type='text')=>{
    const wrap=document.createElement('div'),caption=document.createElement('label'),input=document.createElement(type==='select'?'select':'input');
    caption.className='field-label';caption.htmlFor=prefix+'-'+key;caption.textContent=label;
    input.id=caption.htmlFor;input.className='field-input';if(type!=='select')input.type=type;
    wrap.append(caption,input);host.append(wrap);return input;
  };
  const option=(select,value,label)=>{const opt=document.createElement('option');opt.value=value;opt.textContent=label;select.append(opt);};
  const frequency=field('frequency',mailImageText('Повторение','Repeat'),'select');
  for(const [value,ru,en] of [['','Не повторяется','Does not repeat'],['DAILY','Ежедневно','Daily'],['WEEKLY','Еженедельно','Weekly'],['MONTHLY','Ежемесячно','Monthly'],['YEARLY','Ежегодно','Yearly']])option(frequency,value,mailImageText(ru,en));
  frequency.value=rule.frequency || '';
  const interval=field('interval',mailImageText('Интервал повторения','Repeat interval'),'number');interval.min=1;interval.max=99;interval.step=1;interval.value=rule.interval || 1;
  const dayGroup=document.createElement('div');dayGroup.className='cal-recurrence-days';const dayTitle=document.createElement('span');dayTitle.textContent=mailImageText('Дни недели','Days of the week');dayGroup.append(dayTitle);host.append(dayGroup);
  for(const [value,ru,en] of [['MO','Пн','Mon'],['TU','Вт','Tue'],['WE','Ср','Wed'],['TH','Чт','Thu'],['FR','Пт','Fri'],['SA','Сб','Sat'],['SU','Вс','Sun']]){
    const label=document.createElement('label'),input=document.createElement('input');input.type='checkbox';input.value=value;input.name=prefix+'-day';input.checked=(rule.weekdays || []).includes(value);label.append(input,document.createTextNode(mailImageText(ru,en)));dayGroup.append(label);
  }
  const pattern=field('pattern',mailImageText('Повторять по','Repeat by'),'select');
  for(const [value,ru,en] of [['date','В число первой встречи','First event’s day of month'],['weekday','Порядковому дню недели','Ordinal weekday'],['last-day','Последнему дню месяца','Last day of month']])option(pattern,value,mailImageText(ru,en));
  pattern.value=rule.week_ordinal?'weekday':rule.month_day===-1?'last-day':'date';
  const ordinal=field('ordinal',mailImageText('Какой по счёту','Which occurrence'),'select');
  for(const [value,ru,en] of [[1,'Первый','First'],[2,'Второй','Second'],[3,'Третий','Third'],[4,'Четвёртый','Fourth'],[5,'Пятый','Fifth'],[-1,'Последний','Last']])option(ordinal,String(value),mailImageText(ru,en));
  const weekday=field('weekday',mailImageText('День недели','Weekday'),'select');
  for(const [value,ru,en] of [['MO','Понедельник','Monday'],['TU','Вторник','Tuesday'],['WE','Среда','Wednesday'],['TH','Четверг','Thursday'],['FR','Пятница','Friday'],['SA','Суббота','Saturday'],['SU','Воскресенье','Sunday']])option(weekday,value,mailImageText(ru,en));
  function setDefaultDay(){
    let date=start || $('cal-start')?.value;
    if(!date)return;
    try{
      const zone=info.timezone || timezone;
      if(!/^\d{4}-\d{2}-\d{2}$/.test(date)){
        const parts=new Intl.DateTimeFormat('en',{...(zone&&zone!=='floating'?{timeZone:zone}:{}),year:'numeric',month:'2-digit',day:'2-digit'}).formatToParts(new Date(date));
        const value=key=>parts.find(part=>part.type===key).value;date=value('year')+'-'+value('month')+'-'+value('day');
      }
      const day=new Date(date+'T12:00:00Z');
      ordinal.value=String(Math.floor((day.getUTCDate()-1)/7)+1);weekday.value=['SU','MO','TU','WE','TH','FR','SA'][day.getUTCDay()];
    }catch{}
  }
  setDefaultDay();if(rule.week_ordinal){ordinal.value=String(rule.week_ordinal);weekday.value=rule.weekdays?.[0] || 'MO';}
  const end=field('end',mailImageText('Окончание серии','Ends'),'select');
  for(const [value,ru,en] of [['never','Без окончания','Never'],['count','После числа встреч','After a number of events'],['until','В указанную дату','On a date']])option(end,value,mailImageText(ru,en));
  end.value=rule.count?'count':rule.until?'until':'never';
  const count=field('count',mailImageText('Количество встреч, включая первую','Number of events, including the first'),'number');count.min=1;count.max=1000;count.step=1;count.value=rule.count || 10;
  const until=field('until',mailImageText('Последний день включительно','Last day, inclusive'),'date');until.value=rule.until || '';
  const hint=document.createElement('p');hint.className='meta cal-recurrence-hint';host.append(hint);
  const bookingHint=document.createElement('p');bookingHint.className='meta';bookingHint.textContent=mailImageText('Для бронирования помещения задайте окончание серии: до 1000 встреч в пределах 5 лет. При конфликте ни одна встреча не будет забронирована.','Room bookings require a series end: up to 1000 events within 5 years. A conflict cancels the entire booking.');host.append(bookingHint);
  if(info.exceptions){const note=document.createElement('p');note.className='meta';note.textContent=mailImageText('Отдельно изменённых или отменённых встреч: ','Edited or cancelled occurrences: ')+info.exceptions+'. '+mailImageText('Они сохранятся. Правило, исключающее их исходные даты, будет отклонено.','They are preserved. A rule that removes their original dates will be rejected.');host.append(note);}
  if(info.editable===false){const note=document.createElement('p');note.className='msg';note.textContent=mailImageText('Это правило создано в другом календарном клиенте и не представимо в простом редакторе. Измените его в исходном клиенте; данные сохранены.','This imported rule cannot be represented by the simple editor. Edit it in the original client; its data is preserved.');host.append(note);if(info.raw){const code=document.createElement('code');code.textContent=info.raw;host.append(code);}}
  function update(){
    const active=!!frequency.value,weekly=frequency.value==='WEEKLY',calendarPattern=['MONTHLY','YEARLY'].includes(frequency.value),byWeekday=calendarPattern&&pattern.value==='weekday';
    const units={DAILY:mailImageText('Интервал, дней','Interval, days'),WEEKLY:mailImageText('Интервал, недель','Interval, weeks'),MONTHLY:mailImageText('Интервал, месяцев','Interval, months'),YEARLY:mailImageText('Интервал, лет','Interval, years')};
    interval.previousElementSibling.textContent=units[frequency.value] || mailImageText('Интервал повторения','Repeat interval');
    hint.textContent=mailImageText('Часовой пояс серии: ','Series timezone: ')+(info.timezone==='floating'?mailImageText('локальное время без часового пояса','floating local time'):info.timezone || timezone || 'UTC')+'. '+(weekly?mailImageText('Включите день первой встречи. Без выбора дней используется он.','Include the first event’s weekday. Leaving all days unchecked uses it.'):calendarPattern?mailImageText('Правило должно включать дату первой встречи. Несуществующие даты и пятые дни недели пропускаются. Ежегодное правило действует в месяце первой встречи.','The pattern must include the first event’s date. Nonexistent dates and fifth weekdays are skipped. Yearly patterns use the first event’s month.'):'');
    for(const input of [interval,end]){input.parentElement.hidden=!active;input.disabled=!active;}
    dayGroup.hidden=!weekly;dayGroup.querySelectorAll('input').forEach(input=>input.disabled=!weekly);
    pattern.parentElement.hidden=!calendarPattern;pattern.disabled=!calendarPattern;
    for(const input of [ordinal,weekday]){input.parentElement.hidden=!byWeekday;input.disabled=!byWeekday;}
    count.parentElement.hidden=!active || end.value!=='count';count.disabled=count.parentElement.hidden;count.required=!count.disabled;
    until.parentElement.hidden=!active || end.value!=='until';until.disabled=until.parentElement.hidden;until.required=!until.disabled;
    interval.required=active;hint.hidden=!active;
    if(info.editable===false)host.querySelectorAll('input,select').forEach(input=>input.disabled=true);
  }
  frequency.addEventListener('change',update);end.addEventListener('change',update);pattern.addEventListener('change',()=>{if(!start&&pattern.value==='weekday')setDefaultDay();update();});host.addEventListener('reset-fields',update);update();
  return host;
}

function readRecurrenceFields(prefix) {
  const frequency=$(prefix+'-frequency')?.value || '';
  if(!frequency)return {frequency:'',interval:1};
  const rule={frequency,interval:Number($(prefix+'-interval').value)};
  if(!Number.isInteger(rule.interval)||rule.interval<1||rule.interval>99)throw Error(mailImageText('Интервал должен быть от 1 до 99','Interval must be 1–99'));
  if(frequency==='WEEKLY')rule.weekdays=[...$(prefix).querySelectorAll('input[type=checkbox]:checked')].map(input=>input.value);
  if(['MONTHLY','YEARLY'].includes(frequency)){
    const pattern=$(prefix+'-pattern').value;
    if(pattern==='last-day')rule.month_day=-1;
    if(pattern==='weekday'){rule.week_ordinal=Number($(prefix+'-ordinal').value);rule.weekdays=[$(prefix+'-weekday').value];}
  }
  const end=$(prefix+'-end').value;
  if(end==='count'){rule.count=Number($(prefix+'-count').value);if(!Number.isInteger(rule.count)||rule.count<1||rule.count>1000)throw Error(mailImageText('Укажите от 1 до 1000 встреч','Choose 1–1000 events'));}
  if(end==='until'){rule.until=$(prefix+'-until').value;if(!rule.until)throw Error(mailImageText('Укажите дату окончания','Choose an end date'));}
  return rule;
}

function mountNewEventRecurrence() {
  const form=$('form-cal-event');if(!form)return;
  $('cal-new-recurrence')?.remove();
  const host=createRecurrenceFields('cal-new-recurrence',{},Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC');
  const anchor=$('event-calendar-choice')?.closest('.event-compose-row');
  if(anchor)anchor.after(host);else form.prepend(host);
}

let calRecurrenceLoading=false;
async function openCalRecurrenceEditor() {
  const selected=calState.currentEvent;
  if(!selected || selected.busy_only || !selected.recurrence_organizer || calRecurrenceLoading)return;
  const userID=state.me?.id,objectID=selected.object_id || selected.id;
  calRecurrenceLoading=true;
  const dialog=document.createElement('dialog');dialog.className='calendar-editor-dialog cal-recurrence-dialog';dialog.id='cal-recurrence-dialog';
  const title=document.createElement('h3');title.className='heading';title.id='cal-recurrence-title';title.textContent=mailImageText('Повторение события','Event recurrence');dialog.setAttribute('aria-labelledby',title.id);
  const status=document.createElement('p');status.className='msg';status.id='cal-recurrence-msg';status.setAttribute('role','status');status.textContent=mailImageText('Загрузка…','Loading…');
  const close=document.createElement('button');close.type='button';close.className='btn-secondary';close.textContent=t('cancel');close.onclick=()=>dialog.close();dialog.append(title,status,close);
  let saving=false;dialog.addEventListener('cancel',event=>{if(saving)event.preventDefault();});dialog.addEventListener('close',()=>{dialog.remove();calRecurrenceLoading=false;},{once:true});document.body.append(dialog);dialog.showModal();
  try{
    const event=await api('/api/v1/calendar/events/'+encodeURIComponent(objectID));
    if(!dialog.open || state.me?.id!==userID){dialog.close();return;}
    if(!event.recurrence_organizer)throw Error(mailImageText('Изменять серию может только организатор','Only the organizer can change the series'));
    const context=document.createElement('p');context.className='meta';
    let when=fmtCalEventWhen(event,event.start);try{if(!event.all_day && event.recurrence?.timezone && event.recurrence.timezone!=='floating')when=new Date(event.start).toLocaleString(undefined,{timeZone:event.recurrence.timezone});}catch{}
    context.textContent=event.summary+' · '+mailImageText('Начало серии: ','First event: ')+when;title.after(context);
    const form=document.createElement('form');form.id='form-cal-recurrence';const content=document.createElement('div');content.className='cal-recurrence-content';const fields=createRecurrenceFields('cal-rule',event.recurrence,'',event.all_day?event.start.slice(0,10):event.start);content.append(fields);form.append(content);
    const note=document.createElement('p');note.className='meta';note.textContent=mailImageText('Изменится вся серия. Участникам будут отправлены обновлённые приглашения.','The entire series will change. Attendees will receive updated invitations.');content.append(note);
    const footer=document.createElement('div');footer.className='event-editor-footer';const save=document.createElement('button');save.type='submit';save.className='btn-spray';save.textContent=t('save');save.disabled=event.recurrence?.editable!==true;footer.append(save,close);form.append(status,footer);dialog.append(form);setMsg(status,'');
    form.addEventListener('submit',async e=>{
      e.preventDefault();if(saving || event.recurrence?.editable!==true)return;
      if(state.me?.id!==userID){dialog.close();return;}
      let rule;try{rule=readRecurrenceFields('cal-rule');}catch(error){setMsg(status,error.message,'err');return;}
      if(!form.reportValidity())return;
      saving=true;save.disabled=close.disabled=true;fields.disabled=true;setMsg(status,'…');
      try{
        await api('/api/v1/calendar/events/'+encodeURIComponent(objectID)+'/recurrence',{method:'PUT',body:JSON.stringify({etag:event.etag,rule})});
        dialog.close();if(state.me?.id===userID){calState.eventID='';await loadCalEvents();}
      }catch(error){if(state.me?.id===userID&&dialog.open)setMsg(status,calendarRecurrenceError(error.message),'err');else dialog.close();}
      finally{saving=false;save.disabled=close.disabled=false;fields.disabled=false;}
    });
  }catch(error){if(dialog.open)setMsg(status,error.message,'err');}
}

function calendarRecurrenceError(message) {
  if(message.includes("pattern must include the first event's date"))return mailImageText('Правило должно включать дату первой встречи. Проверьте число, день недели и его порядковый номер; для новой серии измените дату начала.','The pattern must include the first event’s date. Check the day and ordinal; for a new series, adjust the start date.');
  if(message.includes('resource booking requires a recurrence end'))return mailImageText('Для бронирования помещения укажите дату окончания серии или количество встреч.','Choose an end date or occurrence count to book a room.');
  if(message.includes('1000 occurrences within 5 years'))return mailImageText('Можно забронировать до 1000 встреч в пределах 5 лет. Сократите серию.','Book up to 1000 events within 5 years. Shorten the series.');
  if(message.startsWith('resource is busy: ')){
    const detail=message.slice('resource is busy: '.length),match=detail.match(/^(.*) \(([^()]+)\)$/);
    const when=match && new Date(match[2]);
    return mailImageText('Помещение занято: ','Room is busy: ')+(match && !Number.isNaN(when.getTime())?match[1]+' · '+when.toLocaleString():detail)+'. '+mailImageText('Выберите другое время или помещение. Изменения не сохранены.','Choose another time or room. Changes were not saved.');
  }
  if(message.includes('cannot verify availability'))return mailImageText('Не удалось проверить занятость помещения. Бронирование не сохранено.','Room availability could not be verified. The booking was not saved.');
  if(message.includes('use the same resources'))return mailImageText('В каждом повторении должны быть выбраны те же помещения.','Use the same rooms for every occurrence.');
  if(message.includes('edited or cancelled occurrence') || message.includes('separate dates or exceptions'))return mailImageText('Новое правило исключает отдельно изменённые или отменённые встречи. Сохраните их исходные даты в серии.','The new rule removes edited or cancelled occurrences. Keep their original dates in the series.');
  if(message.includes('first event\'s day'))return mailImageText('Включите в дни недели день первой встречи.','Include the first event’s weekday.');
  if(message.includes('recurrence end date'))return mailImageText('Дата окончания должна быть не раньше первой встречи.','The end date must not precede the first event.');

  return message;
}

$('btn-cal-recurrence')?.addEventListener('click',()=>openCalRecurrenceEditor());
