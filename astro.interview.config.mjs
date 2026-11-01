import {defineConfig} from 'astro/config';
export default defineConfig({
  site:'https://jerry-zmf-uestc.github.io',
  base:'/the-way-to-ai-engineer/interview',
  srcDir:'./src/interview-site',
  outDir:'./dist-interview',
  cacheDir:'./.astro-interview',
  publicDir:'./public',
  vite:{server:{strictPort:true,watch:{usePolling:true,interval:250}}},
  markdown:{shikiConfig:{theme:'github-light-default',wrap:false}},
});
