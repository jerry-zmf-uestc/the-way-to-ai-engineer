import { prepareBook } from './book-content.mjs';

const generated = prepareBook();
for (const path of generated) console.log(`generated ${path}`);
console.log(`book content: ${generated.length} published page(s) prepared`);
