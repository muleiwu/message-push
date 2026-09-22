# Architecture Diagrams (archify)

English · [简体中文](README.zh-CN.md)

System architecture diagrams for this repository, generated with [archify](https://github.com/tt-a1i/archify). The Chinese and English versions share the same layout and are generated from the same pipeline, so they stay structurally identical.

## Directory layout

```
docs/archify/
├── zh-CN/                                    # Chinese diagrams
│   ├── message-push-system.architecture.json # editable source (single source of truth)
│   ├── message-push-system.html              # standalone interactive deliverable
│   ├── message-push-system.png               # 4x light export (7456x2776), used by README.zh-CN.md
│   └── message-push-system.visual-check.*    # browser validation evidence (gitignored)
└── en/                                       # English diagrams (same structure, .en suffix)
```

- `*.architecture.json` — the only editable source: a typed-IR spec of components, boundaries, connections, cards, and views.
- `*.html` — the `deliver` output: a self-contained interactive file with theme switching, pan/zoom, and view chapters.
- `*.png` — 4x light-theme raster export referenced by the root README files.
- `*.visual-check.*` — automated browser evidence produced by `visual-check`; gitignored and regenerable at any time.

## The archify skill

[archify](https://github.com/tt-a1i/archify) is an open-source agent skill: a Node.js rendering and validation system that turns a small typed JSON spec into a checked, interactive HTML diagram. Install once per machine:

```bash
npx skills add tt-a1i/archify -g    # installs to ~/.claude/skills/archify
```

The CLI lives at `~/.claude/skills/archify/bin/archify.mjs` (run Node against it; no npm install needed inside the skill).

## Updating the diagrams

Run from the repository root. Set `A=~/.claude/skills/archify`, and replace `<lang>` with `zh-CN` (files without suffix) or `en` (files with `.en` suffix):

```bash
# 1. Edit the source JSON for the language you are changing:
#    docs/archify/<lang>/message-push-system[.en].architecture.json

# 2. Validate; repair only what diagnostics point at (subject + supportedFixes)
node $A/bin/archify.mjs validate architecture \
  docs/archify/en/message-push-system.en.architecture.json --quality standard --json

# 3. Deliver: render + 9 artifact checks + atomic replace of the HTML
node $A/bin/archify.mjs deliver architecture \
  docs/archify/en/message-push-system.en.architecture.json \
  docs/archify/en/message-push-system.en.html --quality standard --json

# 4. Browser evidence (optional; sidecars land next to the HTML)
node $A/bin/archify.mjs visual-check docs/archify/en/message-push-system.en.html --json

# 5. Export the PNG (4x light, overwrites the committed PNG)
node scripts/export-archify-diagram.mjs \
  docs/archify/en/message-push-system.en.html \
  docs/archify/en/message-push-system.en.png 4
```

## Keeping both languages in sync

- Any **geometry change** (positions, sizes, routes, viewBox) must be applied to both JSON files; content is then translated per language.
- Keep code identifiers in English inside localized copy: API paths, protocols, stream names (`push:stream:dead_letter`), product names (Redis, Vue · Vben Admin).
- `meta.locale` (`zh-CN` / `en`) localizes only the viewer chrome — legend, title, a11y, `<html lang>`; it never translates authored content.

## Gotchas learned the hard way

- `via` on a connection must be a **polyline of bend points**, not a single point — a lone point produces a diagonal segment that crosses nodes.
- The readability gate fits the viewBox by **viewport height**: keep the viewBox aspect ratio near ~2.69 (currently 1880x700) or projected node text falls below the floor on 1440x900.
- Label placement tools, in order of preference: default position → `labelDy` / `labelDx` → `labelSegment` → `labelAt`.
- Repair loop discipline: change only the diagnosed subject each round; if two consecutive rounds do not reduce the error count, stop and report the unresolved diagnostics.
