// Package markdowntools writes derived output: it adds missing front matter,
// sets the share permission, quotes broken YAML scalars, and repairs duplicate
// UUIDs in a markdown vault, publishes each document's backlinks as a JSON
// file, copies hashed assets, and writes a static site tree. It reads the
// vault through the markdownindexer package, so scanning and indexing stay
// read-only there while every write lives here.
package markdowntools
