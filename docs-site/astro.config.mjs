import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightImageZoom from 'starlight-image-zoom';
import svelte from '@astrojs/svelte';

export default defineConfig({
  site: 'https://workset.dev',
  integrations: [
    starlight({
      plugins: [starlightImageZoom()],
      title: 'Workset',
      description: 'Multi-repo threads with linked worktrees',
      logo: {
        src: './src/assets/logo.png',
      },
      favicon: '/favicon.png',
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/strantalis/workset',
        },
      ],
      components: {
        ThemeSelect: './src/components/ThemeSelect.astro',
        PageTitle: './src/components/PageTitle.astro',
      },
      editLink: {
        baseUrl: 'https://github.com/strantalis/workset/edit/main/docs-site/',
      },
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Overview', link: '/getting-started/' },
            { label: 'Download', link: '/getting-started/download' },
            { label: 'Quickstart', link: '/getting-started/quickstart' },
            { label: 'Next Steps', link: '/getting-started/next-steps' },
          ],
        },
        {
          label: 'Guides',
          items: [
            { label: 'Desktop App', link: '/guides/desktop-app' },
            { label: 'GitHub Integration', link: '/guides/github-integration' },
            { label: 'Multi-Repo Workflows', link: '/guides/multi-repo-workflows' },
            { label: 'Worksets', link: '/guides/worksets' },
            { label: 'AI Agents', link: '/guides/ai-agents' },
            { label: 'Hooks', link: '/guides/hooks' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'CLI', link: '/reference/cli' },
            { label: 'Config', link: '/reference/config' },
            { label: 'Concepts', link: '/reference/concepts' },
            { label: 'Environment Variables', link: '/reference/env-vars' },
          ],
        },
        { label: 'Troubleshooting', link: '/troubleshooting' },
        { label: 'Contributing', link: '/contributing' },
        { label: 'Changelog', link: '/changelog' },
      ],
      customCss: ['./src/styles/custom.css'],
      head: [
        {
          tag: 'link',
          attrs: {
            rel: 'preconnect',
            href: 'https://fonts.googleapis.com',
          },
        },
        {
          tag: 'link',
          attrs: {
            rel: 'preconnect',
            href: 'https://fonts.gstatic.com',
            crossorigin: '',
          },
        },
        {
          tag: 'link',
          attrs: {
            rel: 'stylesheet',
            href: 'https://fonts.googleapis.com/css2?family=DM+Sans:wght@500;600;700&display=swap',
          },
        },
      ],
    }),
    svelte(),
  ],
});
