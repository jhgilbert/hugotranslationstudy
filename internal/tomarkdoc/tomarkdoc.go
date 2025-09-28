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

	type tItem struct {
		typ   string
		val   []byte
		start int
		end   int
	}
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

	isLeft := func(typ string) bool {
		return typ == "tLeftDelimScNoMarkup" || typ == "tLeftDelimScWithMarkup"
	}
	isRight := func(typ string) bool {
		return typ == "tRightDelimScNoMarkup" || typ == "tRightDelimScWithMarkup"
	}

	// Capture the substring between a left-delim at i and its next right-delim.
	getInterior := func(leftIdx int) (interior string, name string, rightIdx int) {
		j := leftIdx + 1
		for j < len(toks) && !isRight(toks[j].typ) {
			j++
		}
		if j >= len(toks) {
			return "", "", leftIdx
		}
		left := toks[leftIdx]
		right := toks[j]
		inner := body[left.end:right.start]
		trimmed := strings.TrimSpace(inner)

		// name is the first token after trimming optional '/'
		n := trimmed
		if strings.HasPrefix(n, "/") {
			n = strings.TrimSpace(n[1:])
		}
		if sp := strings.Fields(n); len(sp) > 0 {
			name = sp[0]
		}
		return trimmed, name, j
	}

	// Look ahead after the current shortcode to see if a matching close exists (nesting-aware).
	hasMatchingClose := func(fromRightIdx int, name string) bool {
		depth := 0
		for i := fromRightIdx + 1; i < len(toks); i++ {
			if isLeft(toks[i].typ) {
				// closing?
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
				// opening of same name?
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

		switch {
		case t.typ == "tText":
			out.Write(t.val)

		case isLeft(t.typ):
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

		case isRight(t.typ):
			// handled by left-delim branch; ignore stray

		default:
			// Fallback: pass raw bytes (covers names/params we didn't explicitly rebuild)
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