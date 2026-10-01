import { defineConfig } from 'astro/config';

export default defineConfig({
  site: 'https://jerry-zmf-uestc.github.io',
  base: '/the-way-to-ai-engineer',
  markdown: {
    shikiConfig: {
      theme: 'github-dark-default',
      wrap: true,
    },
  },
});
