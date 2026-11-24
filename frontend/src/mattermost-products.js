// Original product interfaces are supplied by the installed plugin bundles.
// Only active products from the current authenticated domain enter the menu.
(function installMattermostProducts(){
 const products=new Map();let catalogToken='';let pending=false;
 function endSession(product){fetch(product.url.replace(/\/(boards|playbooks)$/,'/products/session'),{method:'DELETE',credentials:'same-origin',keepalive:true}).catch(()=>{})}
 function clear(){for(const product of products.values()){endSession(product);product.button.remove();product.section.remove()}products.clear();catalogToken=''}
 async function refresh(){
  const token=state.tokens?.access_token||'';
  if(!token){clear();return}
  if(pending||catalogToken===token)return;pending=true;
  try{
   const response=await fetch('/api/v4/tms/products',{headers:{Authorization:'Bearer '+token}});
   if(!response.ok)return;
   const list=await response.json();if(state.tokens?.access_token!==token)return;
   catalogToken=token;
   for(const [id,existing] of products){if(!list.some(item=>item.id===id&&item.url===existing.url&&item.user_id===existing.user_id)){endSession(existing);existing.button.remove();existing.section.remove();products.delete(id)}}
   const anchor=document.querySelector('[data-app="chat"]');const parent=document.getElementById('app-chat')?.parentElement;
   if(!anchor||!parent)return;
   let lastAnchor=anchor;
   for(const item of list){
    if(products.has(item.id)){lastAnchor=products.get(item.id).button;continue}
    if(!['boards','playbooks'].includes(item.id)||!new RegExp('^/mm/[a-z0-9]{26}/'+item.id+'$').test(item.url))continue;
    if(!APPS.includes(item.id))APPS.push(item.id);
    const button=document.createElement('button');button.type='button';button.className=anchor.className;button.classList.remove('is-active');button.dataset.app=item.id;button.textContent=item.title;button.title=item.title;
    const section=document.createElement('section');section.id='app-'+item.id;section.className='hidden';section.style.cssText='height:100%;min-height:600px;display:flex;flex-direction:column';
    // The global hidden utility must win over inline layout.
    section.style.removeProperty('display');
    const iframe=document.createElement('iframe');iframe.title=item.title;iframe.style.cssText='border:0;width:100%;height:calc(100vh - 100px);min-height:600px';iframe.referrerPolicy='same-origin';section.append(iframe);parent.append(section);
    button.addEventListener('click',()=>showApp(item.id));lastAnchor.parentElement.insertBefore(button,lastAnchor.nextSibling);lastAnchor=button;
    products.set(item.id,{...item,button,section,iframe});
   }
  }catch(error){console.warn('Tayga products discovery failed',error.message)}finally{pending=false}
 }
 const previous=showApp;
 showApp=function(name){previous(name);refresh();const product=products.get(name);if(product){const title=document.getElementById('app-topbar-title');if(title){title.removeAttribute('data-i18n');title.textContent=product.title}if(product.iframe.getAttribute('src')!==product.url)product.iframe.src=product.url;}};
 window.addEventListener('tms-products-session-ended',clear);
 refresh();window.addEventListener('storage',event=>{if(event.key==='tayga.tokens')refresh()});
})();
