const travelView = { offset: 0, owner: '', request: 0 };
function travelText(ru, en) { return mailImageText(ru, en); }
function travelNode(tag, text, className) {
  const node=document.createElement(tag); if(text!==undefined)node.textContent=text;
  if(className)node.className=className; return node;
}
function travelButton(text, action, primary=false) {
  const button=travelNode('button',text,primary?'btn-spray':'btn-secondary');button.type='button';
  button.addEventListener('click',action);return button;
}
function travelTitle(ticket) { return (ticket.kind==='train'?travelText('Поезд ','Train '):travelText('Рейс ','Flight '))+ticket.number; }
function travelStatus(item) {
  if(item.event_missing)return travelText('Событие удалено. Автоматическое восстановление отключено.','Event deleted. It will not be recreated automatically.');
  if(item.state==='saved')return travelText('Добавлено в личный календарь','Added to personal calendar');
  if(item.state==='ignored')return travelText('Пропущено','Dismissed');
  if(item.ticket.cancelled)return travelText('Отмена поездки — проверьте перед применением','Trip cancellation — review before applying');
  if(item.event_id)return travelText('Новые сведения — требуется проверка','Updated itinerary — review required');
  return travelText('Проверьте сведения и часовые пояса','Review details and timezones');
}
async function refreshTravelProfile() {
  const host=$('travel-profile');if(!host)return;
  const request=++travelView.request;const owner=state.tokens?.access_token||'';if(travelView.owner!==owner){travelView.owner=owner;travelView.offset=0;}
  host.replaceChildren(travelNode('h3',travelText('Билеты и поездки','Tickets and trips'),'heading'));
  try {
    const [preferences,result]=await Promise.all([api('/api/v1/travel/preferences'),api('/api/v1/travel?offset='+travelView.offset)]);
    if(travelView.owner!==owner||state.tokens?.access_token!==owner||travelView.request!==request)return;
    host.append(travelNode('p',travelText('Распознавание авиабилетов и железнодорожных билетов в новых письмах и текстовых PDF. События сохраняются в вашем личном календаре.','Recognize flights and train journeys in new mail and text PDFs. Events are saved to your personal calendar.'),'field-hint'));
    const label=travelNode('label',travelText('Обработка новых писем','New mail processing'),'field-label');label.htmlFor='travel-mode';
    const select=travelNode('select',undefined,'field-input');select.id='travel-mode';
    for(const [value,ru,en] of [['off','Выключено','Disabled'],['suggest','Предлагать события','Suggest events'],['auto','Автодобавление при полных данных','Automatically add complete itineraries']]) {
      const option=travelNode('option',travelText(ru,en));option.value=value;select.append(option);
    }
    select.value=preferences.mode||'off';host.append(label,select);
    host.append(travelNode('p',travelText('Для автодобавления нужны маршрут, пассажир, бронь и время с известными часовыми поясами. В квитанциях Авиасейлс пояса SVO и SVX определяются по аэропортам; остальные нужно подтвердить. Неоднозначные данные, изменения и отмены требуют проверки. Старые письма можно обработать кнопкой «Из билета» в письме.','Automatic creation requires a route, passenger, booking reference and times with known timezones. Aviasales receipts resolve SVO and SVX timezones from airport codes; other airports require confirmation. Ambiguous data, changes and cancellations require review. Use “From ticket” in a message to process older mail.'),'field-hint'));
    const msg=travelNode('p','','msg');msg.setAttribute('role','status');
    const save=travelButton(t('save'),async()=>{if(state.tokens?.access_token!==owner)return;save.disabled=true;try{await api('/api/v1/travel/preferences',{method:'PUT',body:JSON.stringify({mode:select.value})});setMsg(msg,t('saved'),'ok');}catch(e){setMsg(msg,e.message,'err');}finally{save.disabled=false;}} ,true);
    const controls=travelNode('div',undefined,'actions');controls.append(save,travelButton(t('refresh'),()=>refreshTravelProfile()));host.append(controls,msg);
    const list=travelNode('div',undefined,'travel-trip-list');list.id='travel-trip-list';
    for(const item of result.candidates||[]) {
      const row=travelNode('article',undefined,'travel-trip-row');
      const text=travelNode('div');text.append(travelNode('strong',travelTitle(item.ticket)+' · '+[item.ticket.origin,item.ticket.destination].filter(Boolean).join(' → ')),travelNode('p',item.ticket.departure.replace('T',' '),'meta'),travelNode('p',travelStatus(item),'meta'));
      if(item.ticket.passenger)text.append(travelNode('p',item.ticket.passenger,'meta'));
      row.append(text,travelButton(travelText('Открыть','Open'),()=>openTravelCandidate(item.id)));list.append(row);
    }
    if(!result.total)list.append(travelNode('p',travelText('Распознанных поездок пока нет.','No recognized trips yet.'),'meta'));
    host.append(list);
    const paging=travelNode('div',undefined,'actions');
    const prev=travelButton(travelText('Назад','Previous'),()=>{travelView.offset=Math.max(0,travelView.offset-10);refreshTravelProfile();});prev.disabled=travelView.offset===0;
    const next=travelButton(travelText('Далее','Next'),()=>{travelView.offset+=10;refreshTravelProfile();});next.disabled=travelView.offset+10>=result.total;
    paging.append(prev,travelNode('span',`${Math.min(travelView.offset+1,result.total||0)}–${Math.min(travelView.offset+10,result.total||0)} / ${result.total||0}`,'meta'),next);host.append(paging);
  }catch(e){host.append(travelNode('p',e.message,'msg err'));}
}
async function openTravelCalendar(item,dialog) {
  dialog?.close();showApp('calendar');await refreshCalendar();await openCalEvent(item.event_id);
}
async function openTravelCandidate(id) {
  const owner=state.tokens?.access_token;
  try{const item=await api('/api/v1/travel/'+encodeURIComponent(id));if(state.tokens?.access_token===owner)showTravelResults([item]);}catch(e){await askAlert(e.message);}
}
async function openTravelDrafts(messageID) {
  const owner=state.tokens?.access_token;
  try {
    const result=await api('/api/v1/mail/messages/'+encodeURIComponent(messageID)+'/travel',{method:'POST'});
    if(state.tokens?.access_token===owner)showTravelResults(result.candidates||[],result.warnings||[]);
  }catch(e){await askAlert(e.message);}
}
function travelOffset(value) {
  if(!/^[+-]\d{2}:\d{2}$/.test(value||''))return '';
  return String((value[0]==='-'?-1:1)*(Number(value.slice(1,3))*60+Number(value.slice(4))));
}
function showTravelResults(items,warnings=[]) {
  const owner=state.tokens?.access_token;
  const dialog=travelNode('dialog',undefined,'protection-reader travel-review');
  const title=travelNode('h3',travelText('Поездки из билетов','Trips from tickets'),'heading');title.id='travel-dialog-title';dialog.setAttribute('aria-labelledby',title.id);dialog.append(title);
  if(!items.length)dialog.append(travelNode('p',travelText('Не удалось распознать поездку. Поддерживаются квитанции Авиасейлс и билеты с явно подписанными полями рейса/поезда и дат. PDF должен содержать текстовый слой.','No trip recognized. Aviasales itineraries and tickets with explicitly labeled flight/train numbers and dates are supported. PDFs must contain text.'),'meta'));
  if(warnings.length)dialog.append(travelNode('p',travelText('Часть содержимого не обработана: для PDF нужен pdftotext; пределы — 4 файла по 8 МиБ, первые 10 страниц. Проверьте оригинал билета.','Some content was not processed: PDFs require pdftotext; limits are 4 files of 8 MiB, first 10 pages. Check the original ticket.'),'msg'));
  for(const item of items) {
    const trip=item.ticket,section=travelNode('section',undefined,'card stack-gap travel-ticket');
    section.append(travelNode('strong',travelTitle(trip)),travelNode('p',[trip.origin,trip.destination].join(' → ')),travelNode('p',travelStatus(item),'meta'));
    if(trip.passenger)section.append(travelNode('p',trip.passenger,'meta'));
    if(trip.departure_zone||trip.arrival_zone)section.append(travelNode('p',travelText('Часовые пояса по аэропортам: ','Airport timezones: ')+[trip.departure_zone,trip.arrival_zone].map(z=>z||travelText('нужно уточнить','confirmation needed')).join(' → '),'field-hint'));
    const details=travelNode('details');details.append(travelNode('summary',travelText('Данные билета','Ticket details')));
    details.append(travelNode('p',[trip.passenger,trip.booking?travelText('Бронь: ','Booking: ')+trip.booking:'',trip.seat?travelText('Место: ','Seat: ')+trip.seat:'',trip.coach?travelText('Вагон: ','Coach: ')+trip.coach:'',travelText('Источник: ','Source: ')+trip.source].filter(Boolean).join(' · '),'meta'));section.append(details);
    if(item.event_id&&!item.event_missing)section.append(travelButton(travelText('Открыть событие','Open event'),()=>openTravelCalendar(item,dialog)));
    if(item.state==='review'&&!item.event_missing) {
      const dates=[],zones=[];
      for(const [index,ru,en] of [[0,'Отправление','Departure'],[1,'Прибытие','Arrival']]) {
        const group=travelNode('div',undefined,'travel-time-row');
        const label=travelNode('label',travelText(ru,en),'field-label');
        const input=travelNode('input',undefined,'field-input');input.type='datetime-local';input.value=index===0?trip.departure:trip.arrival;input.required=true;label.append(input);group.append(label);dates.push(input);
        const zoneLabel=travelNode('label',travelText('Часовой пояс','Timezone'),'field-label');
        const select=travelNode('select',undefined,'field-input');select.required=true;
        const blank=travelNode('option',travelText('Выберите UTC','Choose UTC offset'));blank.value='';select.append(blank);
        const explicit=travelOffset(index===0?trip.departure_offset:trip.arrival_offset);
        const offsets=new Set();for(let n=-720;n<=840;n+=15)offsets.add(n);if(explicit!=='')offsets.add(Number(explicit));
        for(const offset of [...offsets].sort((a,b)=>a-b)) {const option=travelNode('option','UTC'+(offset<0?'-':'+')+String(Math.floor(Math.abs(offset)/60)).padStart(2,'0')+':'+String(Math.abs(offset)%60).padStart(2,'0'));option.value=offset;select.append(option);}
        select.value=explicit;zoneLabel.append(select);group.append(zoneLabel);zones.push(select);section.append(group);
      }
      const msg=travelNode('p','','msg');msg.setAttribute('role','status');
      const save=travelButton(trip.cancelled?travelText('Подтвердить отмену','Confirm cancellation'):item.event_id?travelText('Обновить событие','Update event'):travelText('Добавить в личный календарь','Add to personal calendar'),async()=>{
        if(save.disabled)return;
        if(state.tokens?.access_token!==owner){dialog.close();return;}
        if(dates.some(d=>!d.value)||zones.some(z=>z.value==='')){setMsg(msg,travelText('Укажите даты, время и оба часовых пояса.','Specify dates, times and both timezones.'),'err');return;}
        const utc=(value,offset)=>new Date(Date.parse(value+'Z')-Number(offset)*60000);
        const start=utc(dates[0].value,zones[0].value),end=utc(dates[1].value,zones[1].value);
        if(!Number.isFinite(+start)||!Number.isFinite(+end)||end<=start||end-start>31*86400000){setMsg(msg,travelText('Проверьте время прибытия: оно должно быть после отправления.','Check arrival time: it must follow departure.'),'err');return;}
        save.disabled=true;
        try{const result=await api('/api/v1/travel/'+encodeURIComponent(item.id)+'/event',{method:'POST',body:JSON.stringify({start:start.toISOString(),end:end.toISOString(),etag:item.event_etag})});item.event_id=result.event_id;item.state='saved';dialog.close();if(state.tokens?.access_token!==owner)return;await refreshTravelProfile();if(state.tokens?.access_token===owner)showTravelResults(items,warnings);}
        catch(e){setMsg(msg,e.message,'err');save.disabled=false;}
      },true);
      save.disabled=trip.cancelled&&!item.event_id;
      const ignore=travelButton(travelText('Пропустить','Dismiss'),async()=>{if(ignore.disabled)return;if(state.tokens?.access_token!==owner){dialog.close();return;}ignore.disabled=true;try{await api('/api/v1/travel/'+encodeURIComponent(item.id)+'/ignore',{method:'POST'});item.state='ignored';dialog.close();if(state.tokens?.access_token!==owner)return;await refreshTravelProfile();if(state.tokens?.access_token===owner)showTravelResults(items,warnings);}catch(e){setMsg(msg,e.message,'err');ignore.disabled=false;}});
      const actions=travelNode('div',undefined,'actions');actions.append(save,ignore);section.append(actions,msg);
    }
    dialog.append(section);
  }
  dialog.append(travelButton(t('close'),()=>dialog.close()));dialog.addEventListener('close',()=>dialog.remove());document.body.append(dialog);dialog.showModal();
}
