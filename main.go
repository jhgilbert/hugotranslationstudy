package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gohugoio/hugo/parser/pageparser"
	"gopkg.in/yaml.v3"
)

/*
Round trip + .mdoc with proper shortcode pairing:
1) Parse content/example.md (front matter + body).
2) Emit out/example.json with tokens and byte ranges for text.
3) Read that JSON back.
4) Translate: uppercase all text spans (by byte ranges).
5) Write translated Markdown: content/example.translated.md
6) Convert translated body to .mdoc shortcode syntax (paired vs standalone) and write:
   content/example.translated.mdoc
*/

type Token struct {
	Type  string `json:"type"`
	Val   string `json:"val"`
	Start int    `json:"start"` // byte offset into contentRaw
	End   int    `json:"end"`   // byte offset (exclusive)
}

type TextSpan struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Text  string `json:"text"`
}

type Output struct {
	SourcePath       string                 `json:"sourcePath"`
	FrontMatter      map[string]any         `json:"frontMatter"`
	ContentRaw       string                 `json:"contentRaw"`
	ContentTok       []Token                `json:"contentTokens"`
	ContentTextSpans []TextSpan             `json:"contentTextSpans"`
}

func main() {
	srcPath := "content/example.md"
	outDir := "out"

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", outDir, err)
	}

	// 1–2: parse + write JSON
	jsonPath, outObj := parseAndWriteJSON(srcPath, outDir)

	// 3–4: read JSON + translate (uppercase)
	translatedBody := translateBodyUsingRanges(jsonPath)

	// 5: write translated Markdown (.md)
	mdOut := "content/example.translated.md"
	writeHugoFile(mdOut, outObj.FrontMatter, translatedBody)

	// 6: convert to .mdoc using tokens (paired vs standalone) and write
	mdocBody := convertBodyToMdocTokens(translatedBody)
	mdocOut := "content/example.translated.mdoc"
	writeMdocFile(mdocOut, outObj.FrontMatter, mdocBody)

	fmt.Println("Round trip complete.")
	fmt.Println(" JSON written to:        ", filepath.ToSlash(jsonPath))
	fmt.Println(" Translated .md at:      ", filepath.ToSlash(mdOut))
	fmt.Println(" Translated .mdoc at:    ", filepath.ToSlash(mdocOut))
}

// --- Step 1–2: Parse and JSON ---

func parseAndWriteJSON(srcPath, outDir string) (string, Output) {
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		log.Fatalf("read %s: %v", srcPath, err)
	}

	cf, err := pageparser.ParseFrontMatterAndContent(bytes.NewReader(raw))
	if err != nil {
		log.Fatalf("ParseFrontMatterAndContent: %v", err)
	}

	// Tokenize ONLY the body (no front matter)
	contentRes, err := pageparser.ParseMain(bytes.NewReader(cf.Content), pageparser.Config{})
	if err != nil {
		log.Fatalf("ParseMain(content): %v", err)
	}
	it := contentRes.Iterator()
	src := contentRes.Input()

	var bodyTokens []Token
	var textSpans []TextSpan

	for {
		item := it.Next()
		if item.IsEOF() || item.IsDone() {
			break
		}
		start := item.Pos()
		valB := item.Val(src)
		end := start + len(valB)
		val := string(valB)

		tok := Token{
			Type:  item.Type.String(),
			Val:   val,
			Start: start,
			End:   end,
		}
		bodyTokens = append(bodyTokens, tok)

		if tok.Type == "tText" && len(valB) > 0 {
			textSpans = append(textSpans, TextSpan{
				Start: start,
				End:   end,
				Text:  val,
			})
		}
	}

	out := Output{
		SourcePath:       srcPath,
		FrontMatter:      cf.FrontMatter,
		ContentRaw:       string(cf.Content),
		ContentTok:       bodyTokens,
		ContentTextSpans: textSpans,
	}

	base := strings.TrimSuffix(filepath.Base(srcPath), filepath.Ext(srcPath))
	jsonPath := filepath.Join(outDir, base+".json")
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		log.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
		log.Fatalf("write %s: %v", jsonPath, err)
	}
	fmt.Printf("Wrote %s (%d bytes)\n", filepath.ToSlash(jsonPath), len(data))
	return jsonPath, out
}

// --- Step 3–4: Translate using ranges ---

func translateBodyUsingRanges(jsonPath string) string {
	b, err := os.ReadFile(jsonPath)
	if err != nil {
		log.Fatalf("read %s: %v", jsonPath, err)
	}
	var in Output
	if err := json.Unmarshal(b, &in); err != nil {
		log.Fatalf("unmarshal %s: %v", jsonPath, err)
	}

	body := []byte(in.ContentRaw)
	for i := len(in.ContentTextSpans) - 1; i >= 0; i-- {
		span := in.ContentTextSpans[i]
		if span.Start < 0 || span.End < 0 || span.Start > span.End || span.End > len(body) {
			log.Fatalf("invalid span range: %d..%d (len=%d)", span.Start, span.End, len(body))
		}
		if !utf8.Valid(body[span.Start:span.End]) {
			log.Fatalf("span not valid utf8 at %d..%d", span.Start, span.End)
		}
		upper := strings.ToUpper(string(body[span.Start:span.End]))
		before := append([]byte(nil), body[:span.Start]...)
		after := append([]byte(nil), body[span.End:]...)
		body = append(before, []byte(upper)...)
		body = append(body, after...)
	}
	return string(body)
}

// --- Step 5: Write Markdown (.md) ---

func writeHugoFile(outPath string, frontMatter map[string]any, body string) {
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

// --- Step 6: Convert to .mdoc using tokens (paired vs standalone) ---

type tItem struct {
	typ   string
	val   []byte
	start int
	end   int
}

func convertBodyToMdocTokens(body string) string {
	// Tokenize the *translated* body
	res, err := pageparser.ParseMain(strings.NewReader(body), pageparser.Config{})
	if err != nil {
		log.Fatalf("ParseMain(translated): %v", err)
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
				interior, name, rIdx := getInterior(i)
				_ = interior // not needed; we normalize closing spacing
				if name == "" {
					// Fallback: write original slice if we somehow failed to parse
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
				// Could not parse; conservatively pass through as text-ish
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
			// Normally consumed by the LeftDelim handler; ignore if encountered directly.

		default:
			// Any other token types we didn't explicitly branch on:
			out.Write(t.val)
		}
	}

	return out.String()
}

func writeMdocFile(outPath string, frontMatter map[string]any, body string) {
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
