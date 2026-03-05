package subtokenize

import (
	"bytes"
	"sort"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"golang.org/x/net/html"

	gmext "github.com/yuin/goldmark/extension"
)

// Subtoken is a fine-grained piece of a tText token value.
// Type is "text" (translatable) or "markup" (protected).
type Subtoken struct {
	Type string `json:"type"`
	Val  string `json:"val"`
}

// claimedRange is a byte range in the source with a classification.
type claimedRange struct {
	start int
	stop  int
	typ   string // "text" or "markup"
}

// walker collects claimed byte ranges from the Goldmark AST.
type walker struct {
	source     []byte
	ranges     []claimedRange
	inCodeSpan bool
	inAutoLink bool
}

// Subtokenize parses a tText token value into fine-grained subtokens.
// Translatable text gets type "text"; everything else (markdown syntax,
// HTML tags, code) gets type "markup".
func Subtokenize(source []byte) ([]Subtoken, error) {
	if len(source) == 0 {
		return nil, nil
	}

	md := goldmark.New(
		goldmark.WithExtensions(gmext.NewTable()),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)

	reader := text.NewReader(source)
	doc := md.Parser().Parse(reader)

	w := &walker{source: source}
	w.walk(doc)

	return w.buildSubtokens(), nil
}

// walk traverses the AST depth-first, collecting claimed ranges.
func (w *walker) walk(node ast.Node) {
	switch n := node.(type) {
	case *ast.FencedCodeBlock:
		w.collectLines(n.Lines(), "markup")
		return // don't recurse into children
	case *ast.CodeBlock:
		w.collectLines(n.Lines(), "markup")
		return
	case *ast.HTMLBlock:
		w.collectHTMLBlock(n)
		return
	}

	// Track context for inline elements
	entering := false
	switch node.(type) {
	case *ast.CodeSpan:
		entering = true
		w.inCodeSpan = true
	case *ast.AutoLink:
		entering = true
		w.inAutoLink = true
	}

	// Collect leaf content
	switch n := node.(type) {
	case *ast.Text:
		typ := "text"
		if w.inCodeSpan || w.inAutoLink {
			typ = "markup"
		}
		seg := n.Segment
		if seg.Start < seg.Stop {
			w.ranges = append(w.ranges, claimedRange{
				start: seg.Start,
				stop:  seg.Stop,
				typ:   typ,
			})
		}
	case *ast.RawHTML:
		segs := n.Segments
		for i := 0; i < segs.Len(); i++ {
			seg := segs.At(i)
			if seg.Start < seg.Stop {
				w.ranges = append(w.ranges, claimedRange{
					start: seg.Start,
					stop:  seg.Stop,
					typ:   "markup",
				})
			}
		}
	case *ast.String:
		// String nodes appear in some contexts (e.g., table cells)
		// They hold raw bytes but aren't translatable inline text
		_ = n
	case *east.TableCell:
		// Table cells are containers; recurse via children below
		_ = n
	}

	// Recurse into children
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		w.walk(child)
	}

	// Restore context
	if entering {
		switch node.(type) {
		case *ast.CodeSpan:
			w.inCodeSpan = false
		case *ast.AutoLink:
			w.inAutoLink = false
		}
	}
}

// collectLines adds all lines from a segment collection as claimed ranges.
func (w *walker) collectLines(lines *text.Segments, typ string) {
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		if seg.Start < seg.Stop {
			w.ranges = append(w.ranges, claimedRange{
				start: seg.Start,
				stop:  seg.Stop,
				typ:   typ,
			})
		}
	}
}

// collectHTMLBlock extracts lines from an HTMLBlock, concatenates them,
// and sub-parses with the HTML tokenizer for finer granularity.
func (w *walker) collectHTMLBlock(n *ast.HTMLBlock) {
	lines := n.Lines()
	if lines.Len() == 0 {
		return
	}

	// Find the byte range of the entire HTML block in the source
	firstSeg := lines.At(0)
	lastSeg := lines.At(lines.Len() - 1)
	blockStart := firstSeg.Start
	blockStop := lastSeg.Stop

	// Extract the raw HTML bytes
	raw := w.source[blockStart:blockStop]

	// Sub-parse with HTML tokenizer
	subs := subtokenizeHTML(raw)

	// Convert sub-parsed results into claimed ranges with absolute offsets
	offset := blockStart
	for _, sub := range subs {
		subLen := len(sub.Val)
		if subLen > 0 {
			w.ranges = append(w.ranges, claimedRange{
				start: offset,
				stop:  offset + subLen,
				typ:   sub.Type,
			})
		}
		offset += subLen
	}
}

// buildSubtokens sorts claimed ranges, fills gaps with markup, and merges.
func (w *walker) buildSubtokens() []Subtoken {
	// Sort by start position
	sort.Slice(w.ranges, func(i, j int) bool {
		return w.ranges[i].start < w.ranges[j].start
	})

	var result []Subtoken
	pos := 0

	for _, r := range w.ranges {
		// Skip overlapping or out-of-order ranges
		if r.start < pos {
			// Adjust if partially overlapping
			if r.stop > pos {
				r.start = pos
			} else {
				continue
			}
		}

		// Gap before this range → markup
		if r.start > pos {
			result = append(result, Subtoken{
				Type: "markup",
				Val:  string(w.source[pos:r.start]),
			})
		}

		// The claimed range itself
		if r.start < r.stop {
			result = append(result, Subtoken{
				Type: r.typ,
				Val:  string(w.source[r.start:r.stop]),
			})
		}

		pos = r.stop
	}

	// Trailing gap
	if pos < len(w.source) {
		result = append(result, Subtoken{
			Type: "markup",
			Val:  string(w.source[pos:]),
		})
	}

	// Merge adjacent subtokens of the same type
	return mergeSubtokens(result)
}

// mergeSubtokens combines adjacent subtokens with the same type.
func mergeSubtokens(in []Subtoken) []Subtoken {
	if len(in) == 0 {
		return nil
	}
	out := []Subtoken{in[0]}
	for i := 1; i < len(in); i++ {
		last := &out[len(out)-1]
		if in[i].Type == last.Type {
			last.Val += in[i].Val
		} else {
			out = append(out, in[i])
		}
	}
	return out
}

// subtokenizeHTML splits raw HTML into markup (tags) and text (content).
func subtokenizeHTML(source []byte) []Subtoken {
	tokenizer := html.NewTokenizer(bytes.NewReader(source))
	var result []Subtoken
	consumed := 0

	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			break
		}

		raw := tokenizer.Raw()
		rawStr := string(raw)
		consumed += len(raw)

		switch tt {
		case html.TextToken:
			if len(rawStr) > 0 {
				result = append(result, Subtoken{Type: "text", Val: rawStr})
			}
		default:
			// StartTag, EndTag, SelfClosingTag, Comment, Doctype
			if len(rawStr) > 0 {
				result = append(result, Subtoken{Type: "markup", Val: rawStr})
			}
		}
	}

	// If the tokenizer didn't consume everything (shouldn't happen, but safe)
	if consumed < len(source) {
		result = append(result, Subtoken{Type: "markup", Val: string(source[consumed:])})
	}

	return mergeSubtokens(result)
}
