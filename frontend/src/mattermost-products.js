/* Bridge original products and shortcuts in mail and chat. */
(function installMattermostProducts() {
  const workspace=window.tmsChatWorkspace;
  if(!workspace)return;
  const products=new Map();
  const language=()=>document.documentElement.lang==='en'?'en':'ru';
  const words=(ru,en)=>language()==='en'?en:ru;
  let catalogToken='',pending=null,openDialog=null;

  function syncTheme(iframe) {
    try {
      if(iframe.contentDocument?.documentElement){
        iframe.contentDocument.documentElement.dataset.taygaColor=document.documentElement.dataset.colorMode==='dark'?'dark':'light';
      }
    }catch{ /* Frames must be same-origin to inherit the theme. */ }
  }
  function frameURL(path) {
    const url=new URL(path,location.href);
    if(language()==='en')url.searchParams.set('lang','en');
    else url.searchParams.delete('lang');
    return url.pathname+url.search+url.hash;
  }
  function openFrame(product,path) {
    product.loadedLanguage=language();
    product.iframe.src=frameURL(path);
  }
  function relocalize(product) {
    const {iframe,url}=product;
    iframe.title=product.id==='boards'?words('Доски','Boards'):words('Сценарии','Playbooks');
    if(!iframe.hasAttribute('src')||product.loadedLanguage===language())return;
    let current=iframe.getAttribute('src');
    try {
      const route=new URL(iframe.contentWindow.location.href);
      if(route.origin===location.origin&&(route.pathname===url||route.pathname.startsWith(url+'/')))current=route.href;
    }catch{ /* Keep the last same-origin route. */ }
    openFrame(product,current);
  }
  new MutationObserver(records=>{
    for(const product of products.values()){
      if(records.some(record=>record.attributeName==='lang'))relocalize(product);
      syncTheme(product.iframe);
    }
  }).observe(document.documentElement,{attributes:true,attributeFilter:['data-color-mode','lang']});

  function endSession(product){
    fetch(product.url.replace(/\/(boards|playbooks)$/,'/products/session'),{method:'DELETE',credentials:'same-origin',keepalive:true}).catch(()=>{});
  }
  function clear(){
    if(openDialog?.open)openDialog.close();
    for(const product of products.values()){endSession(product);workspace.unregister(product.id);}
    products.clear();catalogToken='';pending=null;
  }
  function refresh(){
    const token=state.tokens?.access_token||'';
    if(!token){clear();return Promise.resolve();}
    if(catalogToken===token)return Promise.resolve();
    if(pending?.token===token)return pending.promise;
    const request={token};
    pending=request;
    request.promise=(async()=>{
      try{
        const response=await fetch('/api/v4/tms/products',{headers:{Authorization:'Bearer '+token}});
        if(!response.ok)return;
        const list=await response.json();
        if(state.tokens?.access_token!==token||pending!==request)return;
        catalogToken=token;
        for(const [id,existing] of products){
          if(!list.some(item=>item.id===id&&item.url===existing.url&&item.user_id===existing.user_id)){
            endSession(existing);workspace.unregister(id);products.delete(id);
          }
        }
        for(const item of list){
          if(products.has(item.id))continue;
          if(!['boards','playbooks'].includes(item.id)||!new RegExp('^/mm/[a-z0-9]{26}/'+item.id+'$').test(item.url))continue;
          const section=document.createElement('section');section.id='app-'+item.id;section.className='app-page chat-product-page hidden';
          const iframe=document.createElement('iframe');iframe.title=item.title;iframe.className='chat-product-frame';iframe.referrerPolicy='same-origin';
          iframe.addEventListener('load',()=>syncTheme(iframe));section.append(iframe);
          workspace.register(item.id,{label:item.id==='boards'?'Доски':'Сценарии',section});
          const product={...item,section,iframe};products.set(item.id,product);relocalize(product);
        }
      }catch(error){console.warn('Tayga products discovery failed',error.message);}
      finally{if(pending===request)pending=null;}
    })();
    return request.promise;
  }

  function element(tag,text,className){
    const node=document.createElement(tag);if(text!==undefined)node.textContent=text;if(className)node.className=className;return node;
  }
  function button(text,handler){
    const node=element('button',text,'btn-secondary btn-sm');node.type='button';node.addEventListener('click',handler);return node;
  }
  function createTitle(id){
    return id==='boards'?words('Создать доску','Create board'):words('Создать сценарий','Create playbook');
  }
  async function scopedRequest(path,options,product,token) {
    const check=()=>{
      if(state.tokens?.access_token!==token||products.get(product.id)!==product)throw Error(words('Сессия изменилась','Session changed'));
    };
    check();
    const result=await api(product.url.slice(0,-product.id.length-1)+path,{...options,_guard:check});
    check();return result;
  }
  function create(product,sourceTitle) {
    if(openDialog?.open)return;
    const token=state.tokens?.access_token;
    const dialog=element('dialog',undefined,'mm-chat-dialog product-create-dialog');
    openDialog=dialog;
    const form=element('form');
    const titleLabel=element('label',words('Название','Title'),'field-label');
    const title=element('input',undefined,'field-input');
    title.required=true;title.maxLength=120;title.value=sourceTitle.replace(/\s+/g,' ').trim().slice(0,120);
    titleLabel.append(title);
    const descriptionLabel=element('label',words('Описание','Description'),'field-label');
    const description=element('textarea',undefined,'field-input');description.rows=3;description.maxLength=1000;
    descriptionLabel.append(description);
    const controls=element('div',undefined,'actions');
    const cancel=button(words('Отмена','Cancel'),()=>dialog.close());
    const submit=element('button',words('Создать','Create'),'btn-spray');submit.type='submit';
    controls.append(cancel,submit);
    const status=element('p',undefined,'msg');status.setAttribute('role','status');
    form.append(element('h2',createTitle(product.id),'heading'),titleLabel,descriptionLabel,status,controls);
    dialog.append(form);document.body.append(dialog);dialog.showModal();title.focus();title.select();
    dialog.addEventListener('close',()=>{dialog.remove();if(openDialog===dialog)openDialog=null;},{once:true});
    dialog.addEventListener('cancel',event=>{if(submit.disabled)event.preventDefault();});
    form.addEventListener('submit',async event=>{
      event.preventDefault();if(submit.disabled||!title.value.trim())return;
      submit.disabled=cancel.disabled=true;status.textContent='';
      try{
        const call=(path,options)=>scopedRequest(path,options,product,token);
        const {team}=await call('/products/bootstrap');
        const route=await createProductItem(product.id,team.id,title.value.trim(),description.value.trim(),call);
        if(!dialog.open)return;
        dialog.close();
        openFrame(product,product.url+route);
        showApp(product.id);
      }catch(error){
        if(dialog.open)status.textContent=error.data?.message||error.data?.error||error.message;
      }finally{submit.disabled=cancel.disabled=false;}
    });
  }
  async function createProductItem(id,team,title,description,request){
    const post=body=>({method:'POST',body:JSON.stringify(body),headers:{'X-Requested-With':'XMLHttpRequest'}});
    const validID=value=>typeof value==='string'&&/^[a-z0-9]{26,27}$/.test(value);
    if(!validID(team))throw Error(words('Команда недоступна','Team is unavailable'));
    if(id==='playbooks'){
      const playbook=await request('/plugins/playbooks/api/v0/playbooks',post({
        title,description,team_id:team,public:false,reminder_timer_default_seconds:86400,
        checklists:[{title:words('Задачи','Tasks'),items:[]}],
      }));
      if(validID(playbook.id))return '/playbooks/'+playbook.id+'/outline';
    }else{
      const newID=prefix=>prefix+Array.from(crypto.getRandomValues(new Uint8Array(26)),n=>'abcdefghijklmnopqrstuvwxyz234567'[n&31]).join('');
      const boardID=newID('b'),statusID=newID('a'),now=Date.now();
      const result=await request('/plugins/focalboard/api/v2/boards-and-blocks',post({
        boards:[{id:boardID,title,description,showDescription:!!description,teamId:team,type:'P',minimumRole:'',isTemplate:false,properties:{},cardProperties:[{id:statusID,name:words('Статус','Status'),type:'select',options:[]}]}],
        blocks:[{id:newID('v'),boardId:boardID,parentId:boardID,schema:1,type:'view',title:words('Доска','Board view'),createAt:now,updateAt:now,fields:{viewType:'board',groupById:statusID}}],
      }));
      const board=result.boards?.[0];
      if(validID(board?.id))return '/team/'+team+'/'+board.id;
    }
    throw Error(words('Модуль не вернул идентификатор','The product did not return an ID'));
  }

  function appendActions(menu,{title,discuss}) {
    const group=element('div',undefined,'product-source-actions');
    const discussion=button(words('Начать обсуждение в чате','Start a chat discussion'),()=>{
      menu.closest('details')?.removeAttribute('open');
      void discuss();
    });
    const items=new Map(['boards','playbooks'].map(id=>[id,button(createTitle(id),()=>{
      menu.closest('details')?.removeAttribute('open');
      const product=products.get(id);if(product)create(product,title);
    })]));
    group.append(...items.values(),discussion);menu.append(group);
    function update(){
      for(const [id,item] of items){
        item.textContent=createTitle(id);
        item.hidden=!products.has(id);
      }
      discussion.textContent=words('Начать обсуждение в чате','Start a chat discussion');
    }
    const details=menu.closest('details');
    details?.addEventListener('toggle',()=>{if(details.open){update();void refresh().then(update);}});
    update();
  }
  function chatActions(source) {
    const details=element('details',undefined,'chat-source-menu');
    const summary=element('summary','⋯');
    summary.setAttribute('aria-label',words('Действия сообщения','Message actions'));
    const menu=element('div',undefined,'chat-source-menu-panel');
    details.append(summary,menu);appendActions(menu,source);return details;
  }
  window.tmsProductActions={appendActions,chatActions};
  const previous=showApp;
  showApp=function(name){
    previous(name);void refresh();
    const product=products.get(name);
    if(product&&!product.iframe.hasAttribute('src'))openFrame(product,product.url);
  };
  window.addEventListener('tms-products-session-ended',clear);
  void refresh();
  window.addEventListener('storage',event=>{if(event.key==='tayga.tokens')void refresh();});
})();
