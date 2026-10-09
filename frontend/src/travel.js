async function openTravelDrafts(messageID) {
  try {
    const result=await api('/api/v1/mail/messages/'+encodeURIComponent(messageID)+'/travel');
    const trips=result.trips||[];
    if(!trips.length) {await askAlert(mailImageText('Не удалось распознать поездку. Поддерживаются билеты с явно указанными рейсом/поездом и датой отправления, PDF с текстовым слоем. Сканированные билеты пока не поддерживаются.','No trip recognized. Tickets must include an explicit flight/train number and departure date. Text PDFs are supported; scanned tickets are not yet supported.')+(result.warnings?.length?'\n'+result.warnings.join('\n'):''));return;}
    const dialog=document.createElement('dialog');dialog.className='protection-reader';
    const heading=document.createElement('h3');heading.className='heading';heading.textContent=mailImageText('Поездки из билета','Trips from ticket');dialog.append(heading);
    const hint=document.createElement('p');hint.className='meta';hint.textContent=mailImageText('Проверьте сведения и часовые пояса отправления и прибытия. Событие сохраняется после проверки.','Review the data and departure/arrival timezones. The event is saved after review.');dialog.append(hint);
    for(const trip of trips) {
      const section=document.createElement('section');section.className='card stack-gap';
      const title=document.createElement('strong');title.textContent=(trip.kind==='flight'?mailImageText('Рейс ','Flight '):mailImageText('Поезд ','Train '))+trip.number;section.append(title);
      const details=document.createElement('p');details.textContent=[trip.origin+' → '+trip.destination,trip.departure+(trip.arrival?' → '+trip.arrival:''),trip.seat?mailImageText('Место: ','Seat: ')+trip.seat:'',trip.coach?mailImageText('Вагон: ','Coach: ')+trip.coach:''].filter(Boolean).join('\n');details.style.whiteSpace='pre-wrap';section.append(details);
      const zones=[];
      for(const label of [mailImageText('Часовой пояс отправления','Departure timezone'),mailImageText('Часовой пояс прибытия','Arrival timezone')]) {const field=document.createElement('label');field.className='field-label';field.textContent=label;const select=document.createElement('select');select.className='field-input';select.required=true;const blank=document.createElement('option');blank.value='';blank.textContent=mailImageText('Выберите смещение UTC','Choose UTC offset');select.append(blank);for(let offset=-720;offset<=840;offset+=15){const option=document.createElement('option');option.value=offset;option.textContent='UTC'+(offset<0?'-':'+')+String(Math.floor(Math.abs(offset)/60)).padStart(2,'0')+':'+String(Math.abs(offset)%60).padStart(2,'0');select.append(option);}field.append(select);section.append(field);zones.push(select);}
      const open=document.createElement('button');open.type='button';open.className='btn-spray';open.textContent=mailImageText('Создать событие','Create event');
      const status=document.createElement('p');status.className='msg';status.setAttribute('role','status');
      open.addEventListener('click',async()=>{
        if(!zones[0].value||(trip.arrival&&!zones[1].value)) {status.textContent=mailImageText('Укажите часовые пояса.','Specify timezones.');return;}
        const utc=(value,offset)=>new Date(Date.parse(value+'Z')-Number(offset)*60000);
        const start=utc(trip.departure,zones[0].value);const end=trip.arrival?utc(trip.arrival,zones[1].value):null;
        if(end&&end<=start){status.textContent=mailImageText('Прибытие должно быть после отправления. Проверьте часовые пояса.','Arrival must follow departure. Check the timezones.');return;}
        dialog.close();showApp('calendar');await refreshCalendar();startNewCalEvent(trip.departure.slice(0,10));
        $('cal-summary').value=title.textContent+' · '+[trip.origin,trip.destination].filter(Boolean).join(' → ');
        $('cal-location').value=trip.origin;
        const local=date=>{const d=new Date(date.getTime()-date.getTimezoneOffset()*60000);return d.toISOString().slice(0,16)};
        $('cal-start').value=local(start);$('cal-end').value=end?local(end):'';
        $('cal-description').value=[trip.origin+' → '+trip.destination,trip.seat?'Место: '+trip.seat:'',trip.coach?'Вагон: '+trip.coach:'',trip.booking?'Бронирование: '+trip.booking:'',trip.passenger?'Пассажир: '+trip.passenger:'','Источник: '+trip.source,'Часовой пояс отправления: '+zones[0].selectedOptions[0].textContent,trip.arrival?'Часовой пояс прибытия: '+zones[1].selectedOptions[0].textContent:'','Письмо: '+messageID].filter(Boolean).join('\n');
        setMsg($('cal-msg'),end?mailImageText('Проверьте данные билета перед сохранением.','Review ticket data before saving.'):mailImageText('В билете не найдено время прибытия. Укажите окончание поездки.','Arrival time is missing; enter the end time.'));
      });section.append(open,status);dialog.append(section);
    }
    const close=document.createElement('button');close.type='button';close.className='btn-secondary';close.textContent=t('close');close.addEventListener('click',()=>dialog.close());dialog.append(close);dialog.addEventListener('close',()=>dialog.remove());document.body.append(dialog);dialog.showModal();
  }catch(error){await askAlert(error.message);}
}
