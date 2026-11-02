import { defineConfig } from 'astro/config';

export default defineConfig({
  site: 'https://jerry-zmf-uestc.github.io',
  base: '/the-way-to-ai-engineer',
  vite: { server: { strictPort: true, watch: { usePolling: true, interval: 250 } } },
  markdown: {
    shikiConfig: {
      theme: 'github-dark-default',
      wrap: false,
    },
  },
});
