/* Shared navigation for conversations, original products and personal agents. */
window.tmsChatWorkspace = (() => {
  const chat=document.getElementById('app-chat');
  if(!chat)return null;
  const order=['chat','boards','playbooks','agents'],entries=new Map();let active='chat';
  const shell=document.createElement('section');shell.id='app-chat-workspace';shell.className='app-page chat-workspace hidden';
  const header=document.createElement('header');header.className='chat-workspace-header';
  const heading=document.createElement('div');heading.className='chat-workspace-heading';
  const title=document.createElement('strong');title.textContent='Чаты';const subtitle=document.createElement('span');subtitle.textContent='Общение и совместная работа';heading.append(title,subtitle);
  const nav=document.createElement('nav');nav.className='chat-workspace-tabs';nav.setAttribute('role','tablist');nav.setAttribute('aria-label','Разделы чатов');header.append(heading,nav);
  const panels=document.createElement('div');panels.className='chat-workspace-panels';shell.append(header,panels);chat.before(shell);
  const paths={chat:'M4 4h16v12H9l-5 5V4z',boards:'M4 4h16v16H4z M9 4v16 M15 4v16 M4 9h5 M9 14h6 M15 11h5',playbooks:'M8 4h12v16H8 M4 7h1 M4 12h1 M4 17h1 M11 8h6 M11 12h6 M11 16h4',agents:'M12 3l2.2 6.8L21 12l-6.8 2.2L12 21l-2.2-6.8L3 12l6.8-2.2L12 3z'};
  const englishLabels={chat:'Conversations',boards:'Boards',playbooks:'Playbooks',agents:'Agents'};
  function localize(){
    const english=document.documentElement.lang==='en';
    title.textContent=english?'Chats':'Чаты';subtitle.textContent=english?'Messaging and collaboration':'Общение и совместная работа';
    nav.setAttribute('aria-label',english?'Chat sections':'Разделы чатов');
    for(const [id,{button,label,description}] of entries){
      button.lastElementChild.textContent=english?englishLabels[id]:label;
      button.title=english?englishLabels[id]:description;
    }
  }
  new MutationObserver(localize).observe(document.documentElement,{attributes:true,attributeFilter:['lang']});
  function icon(id){const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.setAttribute('viewBox','0 0 24 24');svg.setAttribute('fill','none');svg.setAttribute('stroke','currentColor');svg.setAttribute('stroke-width','1.6');svg.setAttribute('stroke-linecap','round');svg.setAttribute('stroke-linejoin','round');svg.setAttribute('aria-hidden','true');const path=document.createElementNS(svg.namespaceURI,'path');path.setAttribute('d',paths[id]);svg.append(path);return svg;}
  function select(name){active=name;for(const [id,entry] of entries){const selected=id===name;entry.button.setAttribute('aria-selected',String(selected));entry.button.tabIndex=selected?0:-1;entry.button.classList.toggle('is-selected',selected);}}
  function register(id,{label,title=label,section}){
    if(!order.includes(id)||entries.has(id))return;
    if(!APPS.includes(id))APPS.push(id);
    const button=document.createElement('button');button.type='button';button.className='chat-workspace-tab';button.id='chat-workspace-tab-'+id;button.dataset.chatApp=id;button.title=title;button.setAttribute('role','tab');button.setAttribute('aria-controls',section.id);button.append(icon(id));const text=document.createElement('span');text.textContent=label;button.append(text);
    section.setAttribute('role','tabpanel');section.setAttribute('aria-labelledby',button.id);panels.append(section);
    const next=order.slice(order.indexOf(id)+1).map(name=>entries.get(name)?.button).find(Boolean);nav.insertBefore(button,next||null);
    entries.set(id,{button,section,label,description:title});button.onclick=()=>showApp(id);select(active);localize();
  }
  function unregister(id){const entry=entries.get(id);if(!entry)return;entry.button.remove();entry.section.remove();entries.delete(id);if(active===id){const visible=!shell.classList.contains('hidden');select('chat');if(visible&&state.tokens?.access_token)showApp('chat');}}
  nav.addEventListener('keydown',event=>{if(!['ArrowLeft','ArrowRight','Home','End'].includes(event.key))return;const tabs=[...nav.querySelectorAll('[role="tab"]')];const current=tabs.indexOf(event.target.closest('[role="tab"]'));if(current<0)return;event.preventDefault();const index=event.key==='Home'?0:event.key==='End'?tabs.length-1:(current+(event.key==='ArrowRight'?1:-1)+tabs.length)%tabs.length;tabs[index].focus();tabs[index].click();});
  register('chat',{label:'Диалоги',section:chat});
  const previous=showApp;showApp=function(name){const inside=entries.has(name);previous(inside?'chat':name);if(inside)for(const [id,entry]of entries)entry.section.classList.toggle('hidden',id!==name);shell.classList.toggle('hidden',!inside);if(inside){select(name);document.querySelectorAll('.nav-btn[data-app="chat"]').forEach(button=>button.classList.add('is-active'));const top=document.getElementById('app-topbar-title');if(top){top.removeAttribute('data-i18n');top.textContent=title.textContent;}}};
  window.addEventListener('tms-products-session-ended',()=>{shell.classList.add('hidden');select('chat');});
  return {register,unregister,panels};
})();
