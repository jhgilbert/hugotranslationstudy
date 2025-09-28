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

	"hugotranslationstudy/internal/piglatin"
	"hugotranslationstudy/internal/tomarkdoc"

	"github.com/gohugoio/hugo/parser/pageparser"
	"gopkg.in/yaml.v3"
)

/*
A token created by Hugo's pageparser package. For example,
the opening punctuation of a shortcode becomes a token.
*/
type Token struct {
	Type  string `json:"type"`
	Val   string `json:"val"`
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

	// 3–4: read JSON + translate (Pig Latin or whatever your translateBodyUsingRanges does)
	translatedBody := translateBodyUsingRanges(jsonPath)

	// 5: write translated Markdown (.md)
	mdOut := "content/example.translated.md"
	writeHugoFile(mdOut, outObj.FrontMatter, translatedBody)

	// 6: convert the ORIGINAL body to .mdoc (paired vs standalone) and write
	mdocBody := tomarkdoc.ConvertBodyToMdocTokens(outObj.ContentRaw) // <-- original, not translated
	mdocOut := "content/example.mdoc"
	tomarkdoc.WriteMdocFile(mdocOut, outObj.FrontMatter, mdocBody)

	fmt.Println("Round trip complete.")
	fmt.Println(" JSON written to:        ", filepath.ToSlash(jsonPath))
	fmt.Println(" Translated .md at:      ", filepath.ToSlash(mdOut))
	fmt.Println(" Original .mdoc at:      ", filepath.ToSlash(mdocOut))
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

// translateBodyUsingRanges reads the JSON, converts all text spans to Pig Latin,
// and splices them back into the body using byte ranges.
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

		original := string(body[span.Start:span.End])
		translated := piglatin.ToPigLatin(original)

		before := append([]byte(nil), body[:span.Start]...)
		after := append([]byte(nil), body[span.End:]...)
		body = append(before, []byte(translated)...)
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