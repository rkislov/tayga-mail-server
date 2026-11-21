/* Keep the existing rule, vacation and Sieve controllers; compose one settings page. */
(() => {
 const root=$('app-filters'),content=root?.querySelector('.workspace-settings-content');
 if(!content)return;
 const words=workspaceWords,builder=content.querySelector('.visual-rule-builder');
 const vacation=$('vacation-enabled')?.closest('.card'),advanced=content.querySelector('.advanced-sieve');
 if(!builder||!vacation||!advanced)return;
 const outlook=$('outlook-rules-summary');
 content.replaceChildren();
 const heading=document.createElement('header');heading.className='filter-page-heading';
 heading.innerHTML='<span class="filter-page-symbol" aria-hidden="true">'+workspaceIcon('filter')+'</span><div><h2>'+words('Почтовые фильтры','Mail filters')+'</h2><p>'+words('Порядок во входящих — без лишних действий.','Keep your inbox organised, automatically.')+'</p></div>';
 const tabs=document.createElement('div');tabs.className='filter-tabs';tabs.setAttribute('role','tablist');tabs.setAttribute('aria-label',words('Настройки почтовых фильтров','Mail filter settings'));
 const panels=document.createElement('div');panels.className='filter-panels';
 content.append(heading,tabs,panels);
 const sections=[['rules',words('Правила','Rules'),builder],['vacation',words('Автоответ','Auto-reply'),vacation],['advanced',words('Дополнительно','Advanced'),advanced]];
 function selectTab(key,focus=false){
  for(const [name] of sections){const active=name===key,tab=$('filter-tab-'+name);tab.setAttribute('aria-selected',String(active));tab.tabIndex=active?0:-1;$('filter-panel-'+name).hidden=!active;if(active&&focus)tab.focus()}
 }
 for(const [key,label,node] of sections){
  const tab=document.createElement('button');tab.type='button';tab.id='filter-tab-'+key;tab.textContent=label;tab.setAttribute('role','tab');tab.setAttribute('aria-controls','filter-panel-'+key);tab.onclick=()=>selectTab(key);
  tab.onkeydown=event=>{const index=sections.findIndex(section=>section[0]===key);let next;
   if(event.key==='ArrowRight')next=(index+1)%sections.length;else if(event.key==='ArrowLeft')next=(index+sections.length-1)%sections.length;else if(event.key==='Home')next=0;else if(event.key==='End')next=sections.length-1;else return;
   event.preventDefault();selectTab(sections[next][0],true);
  };
  const panel=document.createElement('section');panel.id='filter-panel-'+key;panel.className='filter-panel';panel.setAttribute('role','tabpanel');panel.setAttribute('aria-labelledby',tab.id);panel.tabIndex=0;panel.append(node);tabs.append(tab);panels.append(panel);
 }
 if(outlook)$('filter-panel-rules').append(outlook);
 builder.querySelector('h3').textContent=words('Правила для входящих','Inbox rules');
 builder.querySelector('header .meta').textContent=words('Письма сами попадут в нужную папку. Нажмите на правило, чтобы изменить его.','Send messages to the right folder automatically. Select a rule to edit it.');
 const count=document.createElement('span');count.id='filter-rule-count';count.className='filter-count';builder.querySelector('h3').after(count);
 $('btn-rule-new').innerHTML=workspaceIcon('plus')+'<span>'+words('Создать правило в конструкторе','Create rule in builder')+'</span>';
 const footer=builder.querySelector('footer'),state=document.createElement('span');state.id='filter-save-state';state.setAttribute('role','status');footer.prepend(state);
 $('btn-rules-save').textContent=words('Сохранить изменения','Save changes');
 const reload=$('btn-rules-reload');reload.classList.add('rule-icon');reload.innerHTML=workspaceIcon('refresh');reload.title=words('Обновить правила','Refresh rules');reload.setAttribute('aria-label',reload.title);
 const toggle=$('vacation-enabled'),subject=$('vacation-subject'),body=$('vacation-body'),save=$('btn-vacation-save'),message=$('vacation-msg');
 vacation.classList.add('filter-vacation');vacation.replaceChildren();
 const vacationHead=document.createElement('header');vacationHead.className='filter-vacation-heading';
 vacationHead.innerHTML='<span class="filter-page-symbol" aria-hidden="true">'+workspaceIcon('mail')+'</span><div><h3>'+words('Пока вас нет','While you’re away')+'</h3><p class="meta">'+words('Оставьте сообщение тем, кто напишет вам во время отсутствия.','Leave a message for people who email you while you’re away.')+'</p></div>';
 const toggleLabel=document.createElement('label');toggleLabel.className='filter-vacation-toggle';toggle.classList.add('filter-switch');toggle.setAttribute('role','switch');const toggleText=document.createElement('span');toggleText.textContent=words('Включить автоответ','Enable auto-reply');toggleLabel.append(toggleText,toggle);
 vacation.append(vacationHead,toggleLabel);
 for(const [field,label,placeholder] of [[subject,words('Тема ответа','Reply subject'),words('Например: я в отпуске','For example: out of office')],[body,words('Сообщение','Message'),words('Когда вы вернётесь и с кем можно связаться в ваше отсутствие','When you’ll return and who to contact while you’re away')]]){
  const labelNode=document.createElement('label');labelNode.className='filter-field';labelNode.htmlFor=field.id;const caption=document.createElement('span');caption.textContent=label;labelNode.append(caption,field);field.placeholder=placeholder;vacation.append(labelNode);
 }
 body.rows=6;
 const vacationFooter=document.createElement('footer');vacationFooter.className='filter-vacation-footer';message.setAttribute('role','status');vacationFooter.append(message,save);vacation.append(vacationFooter);
 for(const field of [toggle,subject,body])field.addEventListener('input',()=>setMsg(message,words('Есть несохранённые изменения','Unsaved changes')));
 advanced.open=true;
 advanced.querySelector('summary').textContent=words('Скрипты Sieve','Sieve scripts');
 const advancedHint=document.createElement('p');advancedHint.className='meta';advancedHint.textContent=words('Для сложных сценариев обработки. Активация скрипта заменяет текущие визуальные правила.','For advanced processing. Activating a script replaces the current visual rules.');advanced.querySelector('summary').after(advancedHint);
 selectTab('rules');updateVisualRuleStatus();
})();
