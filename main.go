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
Round trip demo:
- Parse content/example.md (front matter + body)
- Emit out/example.json with tokens and byte ranges for text
- Read the JSON
- Translate: uppercase all text spans (using ranges)
- Rebuild translated Markdown at content/example.translated.md
*/

type Token struct {
	Type  string `json:"type"`
	Val   string `json:"val"`
	Start int    `json:"start"` // byte offset into contentRaw
	End   int    `json:"end"`   // byte offset (exclusive)
}

type TextSpan struct {
	Start int    `json:"start"` // byte offset into contentRaw
	End   int    `json:"end"`   // byte offset (exclusive)
	Text  string `json:"text"`
}

type Output struct {
	SourcePath       string                 `json:"sourcePath"`
	FrontMatter      map[string]any         `json:"frontMatter"`
	ContentRaw       string                 `json:"contentRaw"`       // body only, no FM
	ContentTok       []Token                `json:"contentTokens"`    // all body tokens with ranges
	ContentTextSpans []TextSpan             `json:"contentTextSpans"` // only text tokens with ranges
}

func main() {
	// ---- INPUT/OUTPUT PATHS ----
	srcPath := "content/example.md"
	outDir := "out"

	// Ensure out dir
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", outDir, err)
	}

	// ---- STEP 1 + 2: Parse + write JSON ----
	jsonPath, outObj := parseAndWriteJSON(srcPath, outDir)

	// ---- STEP 3 + 4: Read JSON + translate (uppercase) using ranges ----
	translatedBody := translateBodyUsingRanges(jsonPath)

	// ---- STEP 5: Rebuild translated Hugo file ----
	writeHugoFile("out/example.translated.md", outObj.FrontMatter, translatedBody)

	fmt.Println("Round trip complete.")
	fmt.Println(" JSON written to:   ", filepath.ToSlash(jsonPath))
	fmt.Println(" Translated .md at: ", "out/example.translated.md")
}

// parseAndWriteJSON parses the Hugo file, builds the Output object with ranges,
// writes JSON to out/<base>.json, and returns the JSON path and the Output (for FM reuse).
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

	var (
		bodyTokens []Token
		textSpans  []TextSpan
	)

	for {
		item := it.Next()
		if item.IsEOF() || item.IsDone() {
			break
		}

		start := item.Pos()      // byte offset relative to cf.Content
		valB := item.Val(src)    // []byte slice for token
		end := start + len(valB) // exclusive
		val := string(valB)

		tok := Token{
			Type:  item.Type.String(),
			Val:   val,
			Start: start,
			End:   end,
		}
		bodyTokens = append(bodyTokens, tok)

		// Capture only plain text tokens for translation
		// We avoid relying on unexported constants; match by name:
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

	// Write JSON
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

// translateBodyUsingRanges reads the JSON, uppercases every text span using the recorded byte ranges,
// and returns the translated body string.
func translateBodyUsingRanges(jsonPath string) string {
	b, err := os.ReadFile(jsonPath)
	if err != nil {
		log.Fatalf("read %s: %v", jsonPath, err)
	}
	var in Output
	if err := json.Unmarshal(b, &in); err != nil {
		log.Fatalf("unmarshal %s: %v", jsonPath, err)
	}

	// Apply replacements from last to first so earlier offsets remain valid
	body := []byte(in.ContentRaw)
	for i := len(in.ContentTextSpans) - 1; i >= 0; i-- {
		span := in.ContentTextSpans[i]

		// Defensive checks: ensure offsets are valid byte boundaries
		if span.Start < 0 || span.End < 0 || span.Start > span.End || span.End > len(body) {
			log.Fatalf("invalid span range: %d..%d (len=%d)", span.Start, span.End, len(body))
		}
		if !utf8.Valid(body[span.Start:span.End]) {
			log.Fatalf("span not valid utf8 at %d..%d", span.Start, span.End)
		}

		// Translate: uppercase the exact byte slice
		upper := strings.ToUpper(string(body[span.Start:span.End]))

		// Splice replacement
		before := append([]byte(nil), body[:span.Start]...)
		after := append([]byte(nil), body[span.End:]...)
		body = append(before, []byte(upper)...)
		body = append(body, after...)
	}

	return string(body)
}

// writeHugoFile writes a Hugo content file: front matter as YAML, a fence, then body.
func writeHugoFile(outPath string, frontMatter map[string]any, body string) {
	fm, err := yaml.Marshal(frontMatter)
	if err != nil {
		log.Fatalf("yaml marshal: %v", err)
	}
	buf := &bytes.Buffer{}
	buf.WriteString("---\n")
	buf.Write(fm)
	buf.WriteString("---\n")
	buf.WriteString(body)

	if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
		log.Fatalf("write %s: %v", outPath, err)
	}
}
