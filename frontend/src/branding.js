// Attribution is part of the application, outside all customizable images.
let brandingRevision = 0;
function renderHeaderBranding(value) {
  const image = $('header-branding-image');
  $('header-branding-name').textContent = value.name || 'Tayga Mail';
  if (value.image) image.src = value.image;
  else image.removeAttribute('src');
  image.classList.toggle('hidden', !value.image);
}
function resetBranding() {
  brandingRevision++;
  document.querySelector('dialog.branding-dialog')?.close();
  renderHeaderBranding({});
}
async function refreshBranding() {
  const revision = ++brandingRevision;
  try {
    const value = await api('/api/v1/me/branding');
    if (revision === brandingRevision) renderHeaderBranding(value);
  } catch {
    if (revision === brandingRevision) renderHeaderBranding({});
  }
}

async function openAdminBranding(kind, target) {
  document.querySelector('dialog.branding-dialog')?.close();
  const dialog = directoryDialog(t(kind === 'domains' ? 'branding_domain' : 'branding_tenant') + ' — ' + target.name);
  dialog.classList.add('branding-dialog');
  dialog.append($('branding-template').content.cloneNode(true));
  // Templates are inert until opened; translate the newly inserted elements.
  dialog.querySelectorAll('[data-i18n]').forEach(el => { el.textContent = t(el.dataset.i18n); });
  const el = id => dialog.querySelector('#' + id);
  const form = el('branding-form'), status = el('branding-msg');
  const base = '/api/v1/admin/branding/' + kind + '/' + encodeURIComponent(target.id);
  let draft = {enabled:false, name:'', image:''}, busy = false, reading = false, revision = 0;
  function preview() {
    el('branding-preview-name').textContent = draft.enabled ? draft.name || 'Tayga Mail' : (kind === 'domains' ? t('mig_inherit') : 'Tayga Mail');
    const image = el('branding-preview-image');
    if (draft.enabled && draft.image) image.src = draft.image;
    else image.removeAttribute('src');
    image.classList.toggle('hidden', !draft.enabled || !draft.image);
  }
  function fill(value) {
    draft = value;
    el('branding-name').value = value.name;
    el('branding-enabled').checked = value.enabled;
    el('branding-file').value = '';
    preview();
  }
  function lock(value) {
    busy = value;
    [...form.elements].forEach(control => { control.disabled = value; });
  }
  el('branding-name').oninput = event => { draft.name = event.target.value; preview(); };
  el('branding-enabled').onchange = event => { draft.enabled = event.target.checked; preview(); };
  el('branding-file').onchange = async event => {
    const file = event.target.files[0];
    if (!file) return;
    const read = ++revision;
    reading = true;
    el('branding-form').querySelector('[type=submit]').disabled = true;
    try {
      if (!['image/png', 'image/jpeg'].includes(file.type) || file.size > 1048576) throw Error();
      const bitmap = await createImageBitmap(file);
      const valid = bitmap.width <= 4096 && bitmap.height <= 4096;
      bitmap.close();
      if (!valid) throw Error();
      const image = await new Promise((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(reader.result);
        reader.onerror = reject;
        reader.readAsDataURL(file);
      });
      if (read !== revision || !dialog.isConnected) return;
      draft.image = image;
      draft.enabled = true;
      el('branding-enabled').checked = true;
      preview();
      setMsg(status, '');
    } catch {
      if (read === revision && dialog.isConnected) {
        event.target.value = '';
        setMsg(status, t('branding_invalid'), 'err');
      }
    } finally {
      if (read === revision) { reading = false; el('branding-form').querySelector('[type=submit]').disabled = busy; }
    }
  };
  el('branding-remove-image').onclick = () => {
    revision++; reading = false;
    draft.image = '';
    el('branding-file').value = '';
    form.querySelector('[type=submit]').disabled = busy;
    preview();
  };
  async function save(value) {
    if (busy || reading) return;
    lock(true); setMsg(status, '');
    try {
      const saved = await api(base, {method:'PUT', body:JSON.stringify(value)});
      if (!dialog.isConnected) return;
      fill(saved);
      setMsg(status, t('saved'), 'ok');
      await refreshBranding();
    } catch { if (dialog.isConnected) setMsg(status, t('branding_save_error'), 'err'); }
    finally { lock(false); }
  }
  form.onsubmit = event => { event.preventDefault(); save({...draft, name:draft.name.trim()}); };
  el('branding-reset').onclick = () => {
    revision++; reading = false;
    save({enabled:false, name:'', image:''});
  };
  lock(true);
  try {
    const value = await api(base);
    if (dialog.isConnected) { fill(value); lock(false); }
  } catch (err) { if (dialog.isConnected) setMsg(status, err.message, 'err'); }
}
$('tenant-branding-button')?.addEventListener('click', () => openAdminBranding('tenants', {id:tenantNav.tenantID, name:tenantNav.tenantName}));
