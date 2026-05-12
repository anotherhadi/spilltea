package style

import (
	"github.com/anotherhadi/spilltea/internal/config"

	"charm.land/glamour/v2/ansi"
)

func GlamourStyleConfig(cfg *config.Config) ansi.StyleConfig {
	c := cfg.TUI.Colors

	str := func(s string) *string { return &s }
	hex := func(base string) *string { return str("#" + base) }
	boolPtr := func(b bool) *bool { return &b }
	uintPtr := func(u uint) *uint { return &u }

	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "\n",
				BlockSuffix: "\n",
				Color:       hex(c.Base05),
			},
			Margin: uintPtr(2),
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  hex(c.Base03),
				Italic: boolPtr(true),
			},
			Indent:      uintPtr(1),
			IndentToken: str("│ "),
		},
		List: ansi.StyleList{
			LevelIndent: 2,
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       hex(c.Base0D),
				Bold:        boolPtr(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           hex(c.Base07),
				BackgroundColor: hex(c.Base0D),
				Bold:            boolPtr(true),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "## ",
				Color:  hex(c.Base0D),
				Bold:   boolPtr(true),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "### ",
				Color:  hex(c.Base0C),
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "#### ",
				Color:  hex(c.Base0B),
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "##### ",
				Color:  hex(c.Base09),
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "###### ",
				Color:  hex(c.Base08),
				Bold:   boolPtr(false),
			},
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: boolPtr(true),
		},
		Emph: ansi.StylePrimitive{
			Italic: boolPtr(true),
		},
		Strong: ansi.StylePrimitive{
			Bold: boolPtr(true),
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  hex(c.Base03),
			Format: "\n--------\n",
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
		},
		Task: ansi.StyleTask{
			Ticked:   "[✓] ",
			Unticked: "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Color:     hex(c.Base0C),
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: hex(c.Base0D),
			Bold:  boolPtr(true),
		},
		Image: ansi.StylePrimitive{
			Color:     hex(c.Base0C),
			Underline: boolPtr(true),
		},
		ImageText: ansi.StylePrimitive{
			Color:  hex(c.Base04),
			Format: "Image: {{.text}} ->",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           hex(c.Base0B),
				BackgroundColor: hex(c.Base01),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: hex(c.Base04),
				},
				Margin: uintPtr(2),
			},
			Chroma: &ansi.Chroma{
				Text: ansi.StylePrimitive{
					Color: hex(c.Base05),
				},
				Error: ansi.StylePrimitive{
					Color:           hex(c.Base07),
					BackgroundColor: hex(c.Base08),
				},
				Comment: ansi.StylePrimitive{
					Color:  hex(c.Base03),
					Italic: boolPtr(true),
				},
				CommentPreproc: ansi.StylePrimitive{
					Color: hex(c.Base09),
				},
				Keyword: ansi.StylePrimitive{
					Color: hex(c.Base0E),
				},
				KeywordReserved: ansi.StylePrimitive{
					Color: hex(c.Base0E),
				},
				KeywordNamespace: ansi.StylePrimitive{
					Color: hex(c.Base0D),
				},
				KeywordType: ansi.StylePrimitive{
					Color: hex(c.Base0A),
				},
				Operator: ansi.StylePrimitive{
					Color: hex(c.Base05),
				},
				Punctuation: ansi.StylePrimitive{
					Color: hex(c.Base05),
				},
				Name: ansi.StylePrimitive{
					Color: hex(c.Base05),
				},
				NameBuiltin: ansi.StylePrimitive{
					Color: hex(c.Base0D),
				},
				NameTag: ansi.StylePrimitive{
					Color: hex(c.Base08),
				},
				NameAttribute: ansi.StylePrimitive{
					Color: hex(c.Base0A),
				},
				NameClass: ansi.StylePrimitive{
					Color:     hex(c.Base0A),
					Bold:      boolPtr(true),
					Underline: boolPtr(true),
				},
				NameConstant: ansi.StylePrimitive{
					Color: hex(c.Base09),
				},
				NameDecorator: ansi.StylePrimitive{
					Color: hex(c.Base0C),
				},
				NameFunction: ansi.StylePrimitive{
					Color: hex(c.Base0D),
				},
				LiteralNumber: ansi.StylePrimitive{
					Color: hex(c.Base09),
				},
				LiteralString: ansi.StylePrimitive{
					Color: hex(c.Base0B),
				},
				LiteralStringEscape: ansi.StylePrimitive{
					Color: hex(c.Base0C),
				},
				GenericDeleted: ansi.StylePrimitive{
					Color: hex(c.Base08),
				},
				GenericEmph: ansi.StylePrimitive{
					Italic: boolPtr(true),
				},
				GenericInserted: ansi.StylePrimitive{
					Color: hex(c.Base0B),
				},
				GenericStrong: ansi.StylePrimitive{
					Bold: boolPtr(true),
				},
				GenericSubheading: ansi.StylePrimitive{
					Color: hex(c.Base04),
				},
				Background: ansi.StylePrimitive{
					BackgroundColor: hex(c.Base01),
				},
			},
		},
		Table: ansi.StyleTable{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{},
			},
		},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix: "\n> ",
		},
	}
}
