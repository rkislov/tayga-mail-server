const fs = require('node:fs');
const path = require('node:path');
const source = 'node_modules/ckeditor5';
const dest = '../internal/frontend/dist/assets/ckeditor5';
fs.mkdirSync(path.join(dest, 'translations'), {recursive:true});
for (const file of ['ckeditor5.js', 'ckeditor5.css']) fs.copyFileSync(path.join(source, 'dist/browser', file), path.join(dest, file));
fs.copyFileSync(path.join(source, 'dist/translations/ru.js'), path.join(dest, 'translations/ru.js'));
fs.copyFileSync(path.join(source, 'LICENSE.md'), path.join(dest, 'LICENSE.md'));

fs.copyFileSync(path.join(source, 'COPYING.GPL'), path.join(dest, 'COPYING.GPL'));
