import { defineConfig } from 'vitepress'

// githubSlug mirrors GitHub's heading anchors, so intra-doc links
// written GitHub-style (e.g. #nfr-1-安全模型) keep working on the site.
const githubSlug = (s: string): string =>
  s
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\s-]/gu, '')
    .replace(/\s+/g, '-')

export default defineConfig({
  lang: 'zh-CN',
  title: 'Coterie',
  description: '开源数字服务共享与订阅拼车平台——需求规格、技术设计与项目计划。',
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
    siteTitle: 'Coterie 文档',
    nav: [
      { text: '首页', link: '/' },
      { text: '需求规格', link: '/requirements' },
      { text: '技术设计', link: '/design' },
      { text: '项目计划', link: '/project-plan' },
      { text: '部署', link: '/deployment' },
    ],
    sidebar: [
      {
        text: '开始',
        items: [
          { text: '简介与快速开始', link: '/' },
          { text: '部署', link: '/deployment' },
        ],
      },
      {
        text: '项目文档',
        items: [
          { text: '需求规格', link: '/requirements' },
          { text: '技术设计', link: '/design' },
          { text: '项目计划', link: '/project-plan' },
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
})
