import { withMermaid } from "vitepress-plugin-mermaid";
import { defineConfig } from 'vitepress'

// githubSlug mirrors GitHub's heading anchors, so intra-doc links
// written GitHub-style (e.g. #nfr-1-安全模型) keep working on the site.
const githubSlug = (s: string): string =>
  s
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\s-]/gu, '')
    .replace(/\s+/g, '-')

export default withMermaid(defineConfig({
  // Fleet layout: English lives at the root, Simplified Chinese under /zh/.
  // Only official VitePress locales — no custom switcher, no mirrored
  // file names, no hand-written language banners in the markdown.
  locales: {
    root: { label: "English", lang: "en-US" },
    zh:   { label: "简体中文", lang: "zh-CN", link: "/zh/" }
  },

  lang: 'en-US',
  title: 'Coterie',
  description: 'Open-source platform for shared digital services and subscription carpooling — requirements, technical design and project plan.',
  // GitHub Pages serves the site from the project subpath; every asset
  // URL is prefixed with this or the page 404s.
  base: '/coterie/',
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/coterie/logo.svg' }],
  ],
  markdown: {
    anchor: { slugify: githubSlug },
  },
  themeConfig: {
    locales: {
      root: {
        label: 'English',
        lang: 'en-US',
        siteTitle: 'Coterie Docs',
        nav: [
          { text: 'Home', link: '/' },
          { text: 'Requirements', link: '/requirements' },
          { text: 'Design', link: '/design' },
          { text: 'Project plan', link: '/project-plan' },
          { text: 'Deployment', link: '/deployment' },
        ],
        sidebar: [
          {
            text: 'Start',
            items: [{ text: 'Overview & quick start', link: '/' }],
          },
          {
            text: 'Project docs',
            items: [
              { text: 'Requirements', link: '/requirements' },
              { text: 'Design', link: '/design' },
              { text: 'Project plan', link: '/project-plan' },
            ],
          },
          {
            text: 'Reference',
            items: [
              { text: 'API reference', link: '/api' },
              { text: 'Deployment', link: '/deployment' },
            ],
          },
        ],
        socialLinks: [
          { icon: 'github', link: 'https://github.com/cuihairu/coterie' },
        ],
        outline: { level: [2, 3], label: 'On this page' },
        docFooter: { prevPage: 'Previous page', nextPage: 'Next page' },
        darkModeSwitchLabel: 'Appearance',
        lightModeSwitchTitle: 'Switch to light mode',
        darkModeSwitchTitle: 'Switch to dark mode',
        sidebarMenuLabel: 'Menu',
        returnToTopLabel: 'Back to top',
      },
      zh: {
        label: '简体中文',
        lang: 'zh-CN',
        siteTitle: 'Coterie 文档',
        nav: [
          { text: '首页', link: '/zh/' },
          { text: '需求规格', link: '/zh/requirements' },
          { text: '技术设计', link: '/zh/design' },
          { text: '项目计划', link: '/zh/project-plan' },
          { text: '部署', link: '/zh/deployment' },
        ],
        sidebar: [
          {
            text: '开始',
            items: [{ text: '简介与快速开始', link: '/zh/' }],
          },
          {
            text: '项目文档',
            items: [
              { text: '需求规格', link: '/zh/requirements' },
              { text: '技术设计', link: '/zh/design' },
              { text: '项目计划', link: '/zh/project-plan' },
            ],
          },
          {
            text: '参考',
            items: [
              { text: 'API 参考', link: '/zh/api' },
              { text: '部署', link: '/zh/deployment' },
            ],
          },
        ],
        socialLinks: [
          { icon: 'github', link: 'https://github.com/cuihairu/coterie' },
        ],
        outline: { level: [2, 3], label: '本页目录' },
        docFooter: { prevPage: '上一页', nextPage: '下一页' },
        darkModeSwitchLabel: '外观',
        lightModeSwitchTitle: '切换到浅色模式',
        darkModeSwitchTitle: '切换到深色模式',
        sidebarMenuLabel: '菜单',
        returnToTopLabel: '回到顶部',
      },
    },
  },
}));
