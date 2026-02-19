# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Documentation website foundation using SvelteKit with static adapter, Tailwind CSS v4, and mdsvex (#68)
- Custom rehype plugins for syntax highlighting (highlight.js), heading slugs, and relative link rewriting (#69)
- Responsive layout shell with sidebar navigation, dark terminal-inspired theme, and table of contents (#70)
- Build-time content pipeline reading `docs/*.md` with frontmatter extraction and prerendered static pages (#71)
- Dynamic milestone progress badge on landing page fetched client-side from public GitHub API (#89)

### Changed

- Increased content left padding by 2 Tailwind units across all documentation pages (#89)
- Added `cursor-pointer` to copy button on homepage, accordion panels and filter buttons on examples page (#89)

### Fixed

- Corrected external SimConnect documentation links (Event IDs, SimVars) by removing `/flighting/` from URLs (#89)
