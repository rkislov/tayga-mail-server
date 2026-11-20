// One locally hosted CKEditor build for compose and tenant/domain welcome mail.
let mailEditorLibrary;
async function loadMailEditorLibrary() {
  if (!mailEditorLibrary) mailEditorLibrary = Promise.all([
    import('/assets/ckeditor5/ckeditor5.js'),
    import('/assets/ckeditor5/translations/ru.js')
  ]).catch(error => { mailEditorLibrary = null; throw error; });
  return mailEditorLibrary;
}
async function prepareMailPhoto(file, aborted = () => false) {
  if (!['image/png','image/jpeg','image/gif','image/webp'].includes(file.type) || file.size > 20*1024*1024) throw Error(mailImageText('Выберите фотографию PNG, JPEG, GIF или WebP до 20 МиБ.', 'Choose a PNG, JPEG, GIF or WebP photo up to 20 MiB.'));
  const bitmap = await createImageBitmap(file);
  try {
    if (aborted()) throw new DOMException('Cancelled', 'AbortError');
    const scale = Math.min(1, 1600 / Math.max(bitmap.width, bitmap.height));
    const canvas = document.createElement('canvas'); canvas.width = Math.max(1,Math.round(bitmap.width*scale)); canvas.height = Math.max(1,Math.round(bitmap.height*scale));
    canvas.getContext('2d').drawImage(bitmap,0,0,canvas.width,canvas.height);
    let result = canvas.toDataURL(file.type === 'image/png' ? 'image/png' : 'image/jpeg', .85);
    if (result.length > 2*1024*1024*4/3) result = canvas.toDataURL('image/jpeg', .75);
    if (result.length > 2*1024*1024*4/3) throw Error(mailImageText('Изображение превышает 2 МиБ после обработки.', 'The processed image exceeds 2 MiB.'));
    if (aborted()) throw new DOMException('Cancelled', 'AbortError');
    return result;
  } finally { bitmap.close(); }
}
async function createMailRichEditor(host, {html = '', onChange = () => {}} = {}) {
  const [CK, ru] = await loadMailEditorLibrary();
  const {ClassicEditor, Essentials, Paragraph, Heading, Bold, Italic, Underline, Strikethrough, List, Link, AutoLink, BlockQuote, Table, TableToolbar, Image, ImageToolbar, ImageCaption, ImageStyle, ImageResize, ImageUpload, FileRepository, GeneralHtmlSupport, RemoveFormat} = CK;
  const editor = await ClassicEditor.create(host, {
    licenseKey:'GPL',
    ui:{overlayContainer:host.closest('dialog') || document.body},
    language:typeof lang !== 'undefined' && lang === 'en' ? 'en' : 'ru', translations:[ru.default],
    initialData:html,
    plugins:[Essentials, Paragraph, Heading, Bold, Italic, Underline, Strikethrough, List, Link, AutoLink, BlockQuote, Table, TableToolbar, Image, ImageToolbar, ImageCaption, ImageStyle, ImageResize, ImageUpload, FileRepository, GeneralHtmlSupport, RemoveFormat],
    extraPlugins:[function MailPhotoUpload(editor) {
      editor.plugins.get('FileRepository').createUploadAdapter = loader => {
        let cancelled = false;
        return {upload:async () => ({default:await prepareMailPhoto(await loader.file, () => cancelled)}), abort:() => {cancelled = true;}};
      };
    }],
    toolbar:{items:['undo','redo','|','heading','|','bold','italic','underline','strikethrough','removeFormat','|','link','bulletedList','numberedList','blockQuote','|','uploadImage','insertTable'], shouldNotGroupWhenFull:false},
    image:{upload:{types:['jpeg','png','gif','webp']}, toolbar:['imageTextAlternative','toggleImageCaption','|','resizeImage']},
    table:{contentToolbar:['tableColumn','tableRow','mergeTableCells']},
    htmlSupport:{allow:[{name:/^(p|div|span|blockquote|table|thead|tbody|tfoot|tr|td|th|figure|figcaption|img)$/,attributes:['data-upload-link','data-mail-signature','width','height','colspan','rowspan','align'],styles:['color','background-color','font-size','font-family','text-align','white-space','width','height','max-width','border','border-collapse','padding','margin']}]},
    link:{defaultProtocol:'https://', addTargetToExternalLinks:true}
  });
  editor.model.document.on('change:data', onChange);
  return editor;
}
function mailEditorPending(editor) { return !!editor?.plugins.get('PendingActions')?.hasAny; }

let composeRichEditor = null, composeRichEditorBoot = null, composeRichPendingHTML = null;
async function ensureComposeRichEditor() {
  if (composeRichEditor) return composeRichEditor;
  if (!composeRichEditorBoot) {
    const host = $('compose-body');
    composeRichEditorBoot = createMailRichEditor(host, {html:host.innerHTML, onChange:() => renderComposeAttachments()})
      .then(editor => {
        composeRichEditor = editor;
        if (composeRichPendingHTML !== null) { editor.setData(composeRichPendingHTML); composeRichPendingHTML = null; }
        $('compose-rte-toolbar').hidden = true;
        editor.plugins.get('PendingActions').on('change:hasAny', () => {
          if (!mailEditorPending(editor)) prepareComposeAttachments().catch(error => setMsg($('compose-msg'),error.message,'err'));
          renderComposeAttachments();
        });
        renderComposeAttachments(); return editor;
      }).catch(error => {
        setMsg($('compose-msg'),mailImageText('Не удалось загрузить редактор: ', 'Could not load editor: ')+error.message,'err');
        throw error;
      }).finally(() => { composeRichEditorBoot = null; renderComposeAttachments(); });
  }
  return composeRichEditorBoot;
}
