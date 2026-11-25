// Original product interfaces are supplied by the installed plugin bundles.
// Only active products from the current authenticated domain enter the menu.
(function installMattermostProducts(){
 const workspace=window.tmsChatWorkspace;if(!workspace)return;
 const products=new Map();let catalogToken='';let pending=false;
 function endSession(product){fetch(product.url.replace(/\/(boards|playbooks)$/,'/products/session'),{method:'DELETE',credentials:'same-origin',keepalive:true}).catch(()=>{})}
 function clear(){for(const product of products.values()){endSession(product);workspace.unregister(product.id)}products.clear();catalogToken=''}
 async function refresh(){
  const token=state.tokens?.access_token||'';
  if(!token){clear();return}
  if(pending||catalogToken===token)return;pending=true;
  try{
   const response=await fetch('/api/v4/tms/products',{headers:{Authorization:'Bearer '+token}});
   if(!response.ok)return;
   const list=await response.json();if(state.tokens?.access_token!==token)return;
   catalogToken=token;
   for(const [id,existing] of products){if(!list.some(item=>item.id===id&&item.url===existing.url&&item.user_id===existing.user_id)){endSession(existing);workspace.unregister(id);products.delete(id)}}
   for(const item of list){
    if(products.has(item.id))continue
    if(!['boards','playbooks'].includes(item.id)||!new RegExp('^/mm/[a-z0-9]{26}/'+item.id+'$').test(item.url))continue;
    const section=document.createElement('section');section.id='app-'+item.id;section.className='app-page chat-product-page hidden';
    const iframe=document.createElement('iframe');iframe.title=item.title;iframe.className='chat-product-frame';iframe.referrerPolicy='same-origin';section.append(iframe);
    workspace.register(item.id,{label:item.id==='boards'?'Доски':'Сценарии',title:item.id==='boards'?'Boards — доски задач':'Playbooks — рабочие сценарии',section});
    products.set(item.id,{...item,section,iframe});
   }
  }catch(error){console.warn('Tayga products discovery failed',error.message)}finally{pending=false;if(state.tokens?.access_token&&state.tokens.access_token!==token)queueMicrotask(refresh)}
 }
 const previous=showApp;
 showApp=function(name){previous(name);refresh();const product=products.get(name);if(product){if(product.iframe.getAttribute('src')!==product.url)product.iframe.src=product.url;}};
 window.addEventListener('tms-products-session-ended',clear);
 refresh();window.addEventListener('storage',event=>{if(event.key==='tayga.tokens')refresh()});
})();
