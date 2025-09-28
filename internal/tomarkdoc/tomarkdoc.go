package tomarkdoc

import (
	"bytes"
	"log"
	"os"
	"strings"

	"github.com/gohugoio/hugo/parser/pageparser"
	"gopkg.in/yaml.v3"
)

type tItem struct {
	typ   string
	val   []byte
	start int
	end   int
}

func ConvertBodyToMdocTokens(body string) string {
	res, err := pageparser.ParseMain(strings.NewReader(body), pageparser.Config{})
	if err != nil {
		log.Fatalf("ParseMain(original): %v", err)
	}
	it := res.Iterator()
	src := res.Input()

	var toks []tItem
	for {
		item := it.Next()
		if item.IsEOF() || item.IsDone() {
			break
		}
		start := item.Pos()
		val := item.Val(src)
		toks = append(toks, tItem{
			typ:   item.Type.String(),
			val:   val,
			start: start,
			end:   start + len(val),
		})
	}

	// Helper to capture the interior between a left and the next right delimiter.
	getInterior := func(leftIdx int) (interior string, name string, rightIdx int) {
		// find next right-delim
		j := leftIdx + 1
		for j < len(toks) && toks[j].typ != "tRightDelimScNoMarkup" {
			j++
		}
		if j >= len(toks) {
			// no closing; just treat as text
			return "", "", leftIdx
		}
		left := toks[leftIdx]
		right := toks[j]
		inner := body[left.end:right.start]
		trimmed := strings.TrimSpace(inner)
		// name is the first token after trimming '/'
		n := trimmed
		if strings.HasPrefix(n, "/") {
			n = strings.TrimSpace(n[1:])
		}
		if sp := strings.Fields(n); len(sp) > 0 {
			name = sp[0]
		}
		return trimmed, name, j
	}

	// Look ahead to see if there is a matching close for this name, with nesting.
	hasMatchingClose := func(fromRightIdx int, name string) bool {
		depth := 0
		for i := fromRightIdx + 1; i < len(toks); i++ {
			if toks[i].typ == "tLeftDelimScNoMarkup" {
				// Is it a closing tag?
				if i+1 < len(toks) && toks[i+1].typ == "tScClose" {
					_, closeName, rIdx := getInterior(i)
					if closeName == name {
						if depth == 0 {
							return true
						}
						depth--
					}
					i = rIdx
					continue
				}
				// It's an opening tag; check if same name to manage nesting
				_, openName, rIdx := getInterior(i)
				if openName == name {
					depth++
				}
				i = rIdx
			}
		}
		return false
	}

	var out strings.Builder

	for i := 0; i < len(toks); i++ {
		t := toks[i]

		switch t.typ {
		case "tText":
			out.Write(t.val)

		case "tLeftDelimScNoMarkup":
			// Closing shortcode?
			if i+1 < len(toks) && toks[i+1].typ == "tScClose" {
				_, name, rIdx := getInterior(i)
				if name == "" {
					out.WriteString("{% / %}")
				} else {
					out.WriteString("{% /")
					out.WriteString(name)
					out.WriteString(" %}")
				}
				i = rIdx
				continue
			}

			// Opening shortcode
			interior, name, rIdx := getInterior(i)
			if name == "" {
				out.WriteString("{% ")
				out.WriteString(strings.TrimSpace(interior))
				out.WriteString(" %}")
				i = rIdx
				continue
			}

			// Paired vs standalone?
			if hasMatchingClose(rIdx, name) {
				out.WriteString("{% ")
				out.WriteString(strings.TrimSpace(interior))
				out.WriteString(" %}")
			} else {
				out.WriteString("{% ")
				out.WriteString(strings.TrimSpace(interior))
				out.WriteString(" /%}")
			}
			i = rIdx

		case "tRightDelimScNoMarkup":
			// Normally consumed by the LeftDelim handler; ignore.

		default:
			out.Write(t.val)
		}
	}

	return out.String()
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