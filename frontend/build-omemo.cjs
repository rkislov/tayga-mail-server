const fs=require('node:fs');
fs.copyFileSync('node_modules/libomemo.js/dist/libomemo.esm.min.js','../internal/frontend/dist/assets/libomemo.js');
fs.copyFileSync('node_modules/libomemo.js/LICENSE','../internal/frontend/dist/assets/libomemo-LICENSE.txt');
