# No Frontmatter

This file deliberately has no YAML frontmatter at all. The indexer should
still pick it up: `metadata` becomes `{}` and `id` stays empty.

It does contain one wikilink, [[optimistic-concurrency]], and one web link,
so link extraction still has something to chew on: [example](https://example.com/no-frontmatter).