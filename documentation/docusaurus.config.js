import {fileURLToPath} from 'node:url';
import {repositoryLinks} from './repository-links.js';

const repository = 'https://github.com/mohuganda/moh-sso-dashboard';
const repositoryRoot = fileURLToPath(new URL('../', import.meta.url));

export default {
  title: 'Integrated Health Portal Documentation',
  url: process.env.DOCS_URL ?? 'https://mohuganda.github.io',
  baseUrl: process.env.DOCS_BASE_URL ?? '/docs/',
  trailingSlash: true,
  onBrokenLinks: 'throw',
  markdown: {
    format: 'detect',
    mermaid: true,
    hooks: {
      onBrokenMarkdownLinks: 'throw',
      onBrokenMarkdownImages: 'throw',
    },
  },
  themes: ['@docusaurus/theme-mermaid'],
  presets: [
    [
      'classic',
      {
        docs: {
          path: repositoryRoot,
          include: [
            'docs/**/*.{md,mdx}',
            'frontend/docs/**/*.{md,mdx}',
            'documentation/pages/index.md',
          ],
          routeBasePath: '/',
          sidebarPath: './sidebars.js',
          editUrl: ({docPath}) => `${repository}/edit/main/${docPath}`,
          beforeDefaultRemarkPlugins: [[repositoryLinks, {repository, repositoryRoot}]],
        },
        blog: false,
        pages: false,
        sitemap: false,
      },
    ],
  ],
  themeConfig: {
    navbar: {
      title: 'Integrated Health Portal',
      logo: {
        alt: 'Uganda Ministry of Health',
        src: 'img/moh-logo.png',
      },
      items: [
        {type: 'docSidebar', sidebarId: 'guides', label: 'Documentation', position: 'left'},
        {href: repository, label: 'GitHub', position: 'right'},
      ],
    },
  },
};
