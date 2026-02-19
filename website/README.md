# SimConnect Go SDK -- Documentation Website

Static documentation site for the [SimConnect Go SDK](https://github.com/mrlm-net/simconnect). Built with SvelteKit (static adapter), Tailwind CSS v4, and mdsvex. Renders the `docs/*.md` files from the repository root as browsable pages with syntax highlighting and sidebar navigation.

## Quick Start

```bash
# Install dependencies
npm install

# Start development server (http://localhost:5173)
npm run dev

# Production build (output: website/build/)
npm run build

# Preview the production build locally
npm run preview

# Type-check
npm run check
```

## How It Works

The site reads markdown files from `../docs/` at build time. Each `.md` file becomes a page under `/docs/<slug>`, where the slug is the filename without the `.md` extension. Changes to `docs/*.md` are reflected on the next build or dev server reload -- no manual steps required.

### Content Pipeline

1. `src/lib/content/pipeline.ts` reads all `docs/*.md` files using `gray-matter` for frontmatter extraction
2. Markdown is compiled through mdsvex with three custom rehype plugins (highlight, slug, link rewrite)
3. `src/lib/config/navigation.ts` groups documents by `section` and sorts by `order` to build sidebar navigation
4. SvelteKit prerenders all pages as static HTML at build time

## Adding a New Doc Page

1. Create a markdown file in the repository root `docs/` directory (e.g., `docs/my-topic.md`)
2. Add frontmatter at the top of the file:

```markdown
---
title: "My Topic"
description: "Brief description of what this page covers."
order: 7
section: "client"
---

# My Topic

Content goes here...
```

3. Rebuild or restart the dev server. The page appears at `/docs/my-topic` and in the sidebar.

## Frontmatter Schema

| Field         | Type   | Required | Description                                      |
|---------------|--------|----------|--------------------------------------------------|
| `title`       | string | Yes      | Page title shown in sidebar and page header       |
| `description` | string | Yes      | Brief summary (used in docs listing)              |
| `order`       | number | Yes      | Sort position within its section (lower = higher) |
| `section`     | string | Yes      | Navigation group (see sections below)             |

## Available Sections

| Section      | Sidebar Group       | Default Open |
|--------------|---------------------|--------------|
| `client`     | Client / Engine     | Yes          |
| `manager`    | Manager             | Yes          |
| `events`     | Events              | Yes          |
| `internals`  | Internals           | No           |

To add a new section, update `sectionMeta` and `sectionOrder` in `src/lib/config/navigation.ts`.

## Rehype Plugins

Three custom rehype plugins in `src/lib/plugins/` process markdown during compilation:

- **rehype-highlight.js** -- Syntax highlighting using highlight.js with support for Go, Bash, JSON, YAML, JavaScript, and TypeScript
- **rehype-slug.js** -- Adds `id` attributes to headings for anchor links and table of contents
- **rehype-rewrite-links.js** -- Rewrites relative `.md` links to `/docs/<slug>` format and converts `../examples/*` links to GitHub URLs

## Directory Structure

```
website/
├── package.json             # Dependencies & scripts
├── svelte.config.js         # SvelteKit + mdsvex + rehype pipeline
├── vite.config.js           # Vite build config
├── tsconfig.json            # TypeScript config
├── static/                  # Static assets (favicon, .nojekyll)
├── build/                   # Production output (git-ignored)
└── src/
    ├── app.html             # HTML shell
    ├── app.css              # Tailwind + dark theme tokens
    ├── lib/
    │   ├── plugins/         # Custom rehype plugins
    │   ├── content/         # Build-time content pipeline
    │   ├── components/layout/ # Header, Sidebar, Footer, ToC
    │   ├── config/          # Site config & navigation builder
    │   └── types/           # TypeScript interfaces
    └── routes/
        ├── +layout.svelte   # Root layout (header + sidebar + content)
        ├── +page.svelte     # Landing page
        └── docs/
            ├── +page.svelte # Docs listing
            └── [slug]/      # Individual doc pages
```

## Build Output

`npm run build` produces static HTML in `website/build/`. This directory is git-ignored and suitable for deployment to any static hosting provider.

## GitHub Pages Deployment

Set the `BASE_PATH` environment variable when building for a non-root deployment path:

```bash
BASE_PATH="/simconnect" npm run build
```

This prefixes all internal links and asset paths with the specified base. When deployed at the repository root, leave `BASE_PATH` unset or empty.

## Tech Stack

- [SvelteKit](https://svelte.dev/docs/kit) with `@sveltejs/adapter-static` for static site generation
- [Tailwind CSS v4](https://tailwindcss.com/) with `@tailwindcss/typography` for prose styling
- [mdsvex](https://mdsvex.pngwn.io/) for markdown preprocessing in Svelte
- [highlight.js](https://highlightjs.org/) for syntax highlighting (via custom rehype plugin)
- [gray-matter](https://github.com/jonschlinkert/gray-matter) for YAML frontmatter parsing
