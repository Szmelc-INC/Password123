package cli

// GlamourStyle returns a Markdown style in the project palette (teal accent,
// silver secondary, neutral greys; no purple).
func GlamourStyle() []byte {
	return []byte(`{
  "document": { "block_prefix": "\n", "block_suffix": "\n", "color": "#d0d0d0", "margin": 2 },
  "block_quote": { "indent": 1, "indent_token": "│ ", "color": "#c0c0c0", "italic": true },
  "paragraph": {},
  "list": { "level_indent": 2 },
  "heading": { "block_suffix": "\n", "color": "#1f8a8a", "bold": true },
  "h1": { "prefix": " ", "suffix": " ", "color": "#ffffff", "background_color": "#105c5c", "bold": true },
  "h2": { "prefix": "## " },
  "h3": { "prefix": "### " },
  "h4": { "prefix": "#### " },
  "strong": { "bold": true, "color": "#ffffff" },
  "emph": { "italic": true },
  "strikethrough": { "crossed_out": true },
  "hr": { "color": "#262626", "format": "\n────────\n" },
  "item": { "block_prefix": "• " },
  "enumeration": { "block_prefix": ". " },
  "link": { "color": "#1f8a8a", "underline": true },
  "link_text": { "color": "#c0c0c0", "bold": true },
  "code": { "prefix": " ", "suffix": " ", "color": "#c0c0c0", "background_color": "#1b1b1b" },
  "code_block": { "color": "#c0c0c0", "margin": 2, "chroma": { "text": { "color": "#c0c0c0" } } },
  "table": {},
  "definition_description": { "block_prefix": "\n🠶 " }
}`)
}
