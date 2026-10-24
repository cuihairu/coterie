#!/usr/bin/env node
// check-doc-links verifies every internal link in docs/ resolves to a real
// page, and that every #fragment resolves to a real heading under the site's
// slug algorithm (mirrors docs/.vitepress/config.mts githubSlug and the
// fleet locale layout: English at the root, Chinese under /zh/). It checks
// both body markdown links and VitePress frontmatter link: / href: values
// (hero actions, feature links), which the build renders into the page.
// Exit code 1 on any dead link.

import { readdirSync, readFileSync, existsSync, statSync } from 'node:fs'
import { join, dirname, resolve, extname } from 'node:path'

const docsDir = resolve(process.argv[2] ?? 'docs')

// Same slug rules as config.mts githubSlug.
const slugify = (s) =>
  s
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\s-]/gu, '')
    .replace(/\s+/g, '-')

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    if (name === 'node_modules' || name === '.vitepress' || name === 'dist') continue
    const st = statSync(p)
    if (st.isDirectory()) walk(p, out)
    else if (extname(p) === '.md') out.push(p)
  }
  return out
}

// Heading slugs per file, computed lazily.
const headingCache = new Map()
function headingsOf(file) {
  if (!headingCache.has(file)) {
    const slugs = new Set()
    for (const line of readFileSync(file, 'utf8').split('\n')) {
      const m = line.match(/^(#{1,6})\s+(.*)$/)
      if (m) slugs.add(slugify(m[2].replace(/[#\s]+$/, '')))
    }
    headingCache.set(file, slugs)
  }
  return headingCache.get(file)
}

// Site-absolute path (/foo, /zh/foo) -> candidate files under docs/.
// VitePress resolves /foo to foo.md or foo/index.md.
function absoluteCandidates(linkPath) {
  const rel = linkPath.replace(/^\/+/, '')
  return [
    join(docsDir, `${rel}.md`),
    join(docsDir, rel, 'index.md'),
    join(docsDir, rel),
  ]
}

function relativeCandidates(fromFile, linkPath) {
  const base = resolve(dirname(fromFile), linkPath)
  return [base, `${base}.md`, join(base, 'index.md')]
}

function resolveTarget(fromFile, target) {
  const [pathPart] = target.split('#')
  const candidates = target.startsWith('/')
    ? absoluteCandidates(pathPart)
    : relativeCandidates(fromFile, pathPart)
  return candidates.find((c) => existsSync(c) && statSync(c).isFile())
}

// { line, target } pairs from one file's body plus its frontmatter.
function linksIn(file) {
  const lines = readFileSync(file, 'utf8').split('\n')
  const found = []
  const linkRe = /\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g
  const fmRe = /^\s*(?:link|href)\s*:\s*["']?([^"'\s#][^"'\s]*)/

  // Frontmatter is the block between the first two --- fences, if the file
  // starts with one; link: / href: values in it are rendered as page links.
  let inFrontmatter = false
  let fences = 0
  let inFence = false

  lines.forEach((line, i) => {
    if (/^```/.test(line.trim())) {
      inFence = !inFence
      return
    }
    if (inFence) return
    if (line.trim() === '---') {
      fences++
      inFrontmatter = fences === 1 ? true : fences === 2 ? false : inFrontmatter
      return
    }
    const m = fmRe.exec(line)
    if (inFrontmatter && m) {
      found.push({ line: i + 1, target: m[1] })
      return
    }
    linkRe.lastIndex = 0
    let body
    while ((body = linkRe.exec(line)) !== null) found.push({ line: i + 1, target: body[1] })
  })
  return found
}

const files = walk(docsDir)
let dead = 0

for (const file of files) {
  for (const { line, target } of linksIn(file)) {
    if (/^(https?:|mailto:|#)/.test(target)) continue
    const [pathPart, frag] = target.split('#')
    if (pathPart === '') {
      // Same-page anchor.
      if (frag && !headingsOf(file).has(frag)) {
        dead++
        console.log(`${file}:${line}: dead anchor #${frag}`)
      }
      continue
    }
    const resolved = resolveTarget(file, target)
    if (!resolved) {
      dead++
      console.log(`${file}:${line}: dead link (${target})`)
      continue
    }
    if (frag && extname(resolved) === '.md' && !headingsOf(resolved).has(frag)) {
      dead++
      console.log(`${file}:${line}: dead anchor ${frag} in ${target}`)
    }
  }
}

console.log(`checked ${files.length} files, dead links: ${dead}`)
process.exit(dead === 0 ? 0 : 1)
