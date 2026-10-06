---
title: [unclosed flow sequence
tags: nobody-validates-this
---

This file has an intentionally broken YAML frontmatter block (an unterminated
flow sequence). The scanner tolerates it: frontmatter stays `null`, a
`parseError` is recorded, and the body is still indexed with its wikilinks,
like [[full-monty]].