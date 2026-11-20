/* Protection overview, quarantine and policy controls use separate calm panels. */
(() => {
  const card=document.querySelector('#app-antispam>.card');
  const settings=card?.querySelector('.protection-settings');
  if(!card||!settings)return;
  const words=workspaceWords,existing=$('protection-diagnostics');
  const quarantine=document.createElement('section');quarantine.id='protection-quarantine';
  const tail=[];for(let node=settings.nextSibling;node;node=node.nextSibling)if(node!==existing)tail.push(node);
  quarantine.append(...tail);
  const overview=document.createElement('section');overview.id='protection-overview';if(existing)overview.append(existing);
  const policy=document.createElement('section');policy.id='protection-policy';
  const heading=document.createElement('h3');heading.textContent=words('Правила защиты','Protection policies');
  const note=document.createElement('p');note.className='meta';note.textContent=words('Настройки проверки входящих писем и обработки нежелательной почты.','Configure incoming message checks and unwanted mail handling.');
  policy.append(heading,note,settings);
  const tabs=document.createElement('nav');tabs.className='workspace-view-tabs protection-view-tabs';tabs.setAttribute('role','tablist');tabs.setAttribute('aria-label',words('Разделы антиспама','Protection sections'));
  const panels=[overview,quarantine,policy],labels=[words('Обзор','Overview'),words('Карантин','Quarantine'),words('Настройки','Settings')];
  function select(index,focus=false){panels.forEach((panel,i)=>{panel.hidden=i!==index;const tab=tabs.children[i];tab.setAttribute('aria-selected',String(i===index));tab.setAttribute('aria-pressed',String(i===index));tab.tabIndex=i===index?0:-1});if(focus)tabs.children[index].focus()}
  panels.forEach((panel,index)=>{panel.setAttribute('role','tabpanel');panel.setAttribute('aria-labelledby','protection-tab-'+index);const tab=document.createElement('button');tab.type='button';tab.id='protection-tab-'+index;tab.setAttribute('role','tab');tab.setAttribute('aria-controls',panel.id);tab.textContent=labels[index];tab.onclick=()=>select(index);tab.onkeydown=e=>{let next=index;if(e.key==='ArrowRight')next=(index+1)%3;else if(e.key==='ArrowLeft')next=(index+2)%3;else if(e.key==='Home')next=0;else if(e.key==='End')next=2;else return;e.preventDefault();select(next,true)};tabs.append(tab)});
  card.append(tabs,...panels);select(0);
})();
