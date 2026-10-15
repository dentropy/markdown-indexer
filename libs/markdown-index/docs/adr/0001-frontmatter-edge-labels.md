# Front matter edges use their own labels

Front matter used to produce no edges. Wikilinks, markdown links, and `#name` tags found in front matter strings are now edges, labeled `FRONTMATTER_INTERNAL`, `FRONTMATTER_WEBSITE`, and `FRONTMATTER_TAG`. The label is part of the edge's content address, so a new label keeps every existing body edge address stable. Reusing `INTERNAL`, `WEBSITE`, and `TAG` would have merged a front matter hit with the same body hit.
