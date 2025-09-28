package tomarkdoc

import (
	"bytes"
	"log"
	"os"
	"strings"

	"github.com/gohugoio/hugo/parser/pageparser"
	"gopkg.in/yaml.v3"
)

// Tok is a tiny wrapper around pageparser items that we care about.
type Tok struct {
	Typ   string
	Val   []byte
	Start int
	End   int
}

// Public entry point: convert a Hugo body to .mdoc shortcode punctuation.
func ConvertBodyToMdocTokens(body string) string {
	toks := tokenizeShortcodes(body)
	return renderToMdoc(toks, body)
}

/* ------------------------------- Tokenizing ------------------------------- */

func tokenizeShortcodes(body string) []Tok {
	res, err := pageparser.ParseMain(strings.NewReader(body), pageparser.Config{})
	if err != nil {
		log.Fatalf("ParseMain(original): %v", err)
	}
	it := res.Iterator()
	src := res.Input()

	var toks []Tok
	for {
		item := it.Next()
		if item.IsEOF() || item.IsDone() {
			break
		}
		start := item.Pos()
		val := item.Val(src)
		toks = append(toks, Tok{
			Typ:   item.Type.String(),
			Val:   val,
			Start: start,
			End:   start + len(val),
		})
	}
	return toks
}

func isLeftDelim(typ string) bool {
	return typ == "tLeftDelimScNoMarkup" || typ == "tLeftDelimScWithMarkup"
}
func isRightDelim(typ string) bool {
	return typ == "tRightDelimScNoMarkup" || typ == "tRightDelimScWithMarkup"
}

/* -------------------------------- Parsing -------------------------------- */

// getInterior returns the raw string between a left delimiter at leftIdx and
// its immediate right delimiter, plus the parsed shortcode name and the index
// of that right delimiter in toks.
func getInterior(toks []Tok, body string, leftIdx int) (interior, name string, rightIdx int) {
	j := leftIdx + 1
	for j < len(toks) && !isRightDelim(toks[j].Typ) {
		j++
	}
	if j >= len(toks) {
		return "", "", leftIdx // no closing; treat as text
	}
	left := toks[leftIdx]
	right := toks[j]
	inner := body[left.End:right.Start]
	trimmed := strings.TrimSpace(inner)

	// The shortcode name is the first field after trimming optional leading "/".
	n := trimmed
	if strings.HasPrefix(n, "/") {
		n = strings.TrimSpace(n[1:])
	}
	if sp := strings.Fields(n); len(sp) > 0 {
		name = sp[0]
	}
	return trimmed, name, j
}

// hasMatchingClose checks if, after the right delimiter of an opening tag,
// a matching closing tag for the given name occurs (nesting-aware).
func hasMatchingClose(toks []Tok, body string, fromRightIdx int, name string) bool {
	depth := 0
	for i := fromRightIdx + 1; i < len(toks); i++ {
		if isLeftDelim(toks[i].Typ) {
			// Closing tag?
			if i+1 < len(toks) && toks[i+1].Typ == "tScClose" {
				_, closeName, rIdx := getInterior(toks, body, i)
				if closeName == name {
					if depth == 0 {
						return true
					}
					depth--
				}
				i = rIdx
				continue
			}
			// Opening of the same name (nested)
			_, openName, rIdx := getInterior(toks, body, i)
			if openName == name {
				depth++
			}
			i = rIdx
		}
	}
	return false
}

/* -------------------------------- Rendering ------------------------------- */

func renderToMdoc(toks []Tok, body string) string {
	var out strings.Builder

	for i := 0; i < len(toks); i++ {
		t := toks[i]

		switch {
		case t.Typ == "tText":
			out.Write(t.Val)

		case isLeftDelim(t.Typ):
			// Closing shortcode?
			if i+1 < len(toks) && toks[i+1].Typ == "tScClose" {
				writeClosingShortcode(&out, toks, body, &i)
				continue
			}
			// Opening shortcode (paired vs standalone)
			writeOpeningShortcode(&out, toks, body, &i)

		case isRightDelim(t.Typ):
			// Right delimiters are consumed by left handlers; ignore stray.

		default:
			// Fallback: pass raw bytes (covers any token we didn't model).
			out.Write(t.Val)
		}
	}

	return out.String()
}

func writeClosingShortcode(out *strings.Builder, toks []Tok, body string, i *int) {
	_, name, rIdx := getInterior(toks, body, *i)
	if name == "" {
		out.WriteString("{% / %}")
	} else {
		out.WriteString("{% /")
		out.WriteString(name)
		out.WriteString(" %}")
	}
	*i = rIdx // advance past the right delimiter we consumed
}

func writeOpeningShortcode(out *strings.Builder, toks []Tok, body string, i *int) {
	interior, name, rIdx := getInterior(toks, body, *i)
	trimmed := strings.TrimSpace(interior)

	if name == "" {
		// Could not parse a name; pass interior through with normalized spacing.
		out.WriteString("{% ")
		out.WriteString(trimmed)
		out.WriteString(" %}")
		*i = rIdx
		return
	}

	if hasMatchingClose(toks, body, rIdx, name) {
		// Paired shortcode
		out.WriteString("{% ")
		out.WriteString(trimmed)
		out.WriteString(" %}")
	} else {
		// Standalone (self-closing in .mdoc)
		out.WriteString("{% ")
		out.WriteString(trimmed)
		out.WriteString(" /%}")
	}
	*i = rIdx
}

func WriteMdocFile(outPath string, frontMatter map[string]any, body string) {
	// Front matter identical (YAML fences)
	fm, err := yaml.Marshal(frontMatter)
	if err != nil {
		log.Fatalf("yaml marshal: %v", err)
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(fm)
	buf.WriteString("---\n")
	buf.WriteString(body)

	if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
		log.Fatalf("write %s: %v", outPath, err)
	}
}