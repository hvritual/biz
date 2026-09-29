# better-typography in biz

For UI implementation and review, first read the root and web AGENTS, the current generated design index/Page Contract, this file, then the local SKILL.md and the relevant local reference documents. Do not run an unpinned npx installer in CI.

## Project precedence

The vendored upstream Markdown is immutable, verified against SOURCE.json Git blob hashes and licensed under the adjacent MIT LICENSE. Project-specific decisions belong here, never as silent edits to upstream files. This is a repository-level Agent Skill, not a claim of global ChatGPT plugin installation.

The current web/ui-contracts.json declares CoffeeLink V1.2. Preserve the existing approved tokens, system sans-serif stack, Vue component architecture and joined navigation geometry. Do not downgrade it to V1.1 just because #296 used the older name. Upstream size/line-height examples are not permission to replace the product type scale or introduce fonts. No font binaries, new UI framework or production package dependency is added.

Prefer genuinely 16px mobile inputs over transform-scaling small text. Do not disable user zoom. Use feature detection/progressive enhancement for newer text CSS rather than assuming the historical browser-version notes remain current. Verify CJK measure, mixed Chinese/Latin IDs and glyph coverage; 65ch is not 65 Chinese characters. Never force Latin negative letter-spacing or heading leading onto wrapped CJK copy without visual evidence.

## UI evidence required by #296

Record candidate SHA, upstream source commit, inspected routes/components, actual rendered viewports (1366x768, 1440x900, 1536x1024, 390x844), browser zoom at 200%, long Chinese/English/identifier cases and changing numbers. Record HIGH/MEDIUM/LOW findings, source locations, before/after and rationale. Mark unexecuted checks Not verified. Real browser zoom is not deviceScaleFactor.

Check role-based text tokens; descending heading hierarchy; readable wrapped explanations; truncation with a complete accessible value; non-jumping numeric columns; full state/error/recovery copy; mobile form text and focus. Required price, quota, expiry and destructive-action consequences must not be hidden in tooltips. Preserve units, currency, scope and unknown states. Static code/hash checks cannot approve rendered typography.

`python3 scripts/check_ui_skill_source.py` verifies the installed source files and references. Its PASS means provenance consistency only, not that an Agent actually read the skill or that a page was visually approved. Docs-only and backend-only changes do not invent screenshots to satisfy a UI requirement.
