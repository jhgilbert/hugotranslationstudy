package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gohugoio/hugo/parser/pageparser"
)

type Token struct {
	Type  string `json:"type"`
	Val   string `json:"val"`
	Start int    `json:"start"` // byte offset into contentRaw
	End   int    `json:"end"`   // byte offset into contentRaw (exclusive)
}

type TextSpan struct {
	Start int    `json:"start"` // byte offset into contentRaw
	End   int    `json:"end"`   // byte offset into contentRaw (exclusive)
	Text  string `json:"text"`
}

type Output struct {
	SourcePath        string                 `json:"sourcePath"`
	FrontMatter       map[string]any         `json:"frontMatter"` // decoded FM only
	ContentRaw        string                 `json:"contentRaw"`  // raw Markdown body only (no FM)
	ContentTok        []Token                `json:"contentTokens"`
	ContentTextSpans  []TextSpan             `json:"contentTextSpans"` // only text tokens (easy to translate)
}

func main() {
	srcPath := "content/example.md"
	outDir := "out"

	// Read the whole file
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		log.Fatalf("read %s: %v", srcPath, err)
	}

	// 1) Split into Front Matter (decoded) and Content (raw body)
	cf, err := pageparser.ParseFrontMatterAndContent(bytes.NewReader(raw))
	if err != nil {
		log.Fatalf("ParseFrontMatterAndContent: %v", err)
	}

	// 2) Tokenize ONLY the content body (so FM never appears here)
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

		start := item.Pos()                 // byte offset into cf.Content
		valB := item.Val(src)               // []byte slice for this token
		end := start + len(valB)            // exclusive
		val := string(valB)                 // keep exact text; do not truncate

		tok := Token{
			Type:  item.Type.String(),
			Val:   val,
			Start: start,
			End:   end,
		}
		bodyTokens = append(bodyTokens, tok)

		// Collect only text tokens as spans for translation workflows
		if item.IsText() && len(val) > 0 {
			textSpans = append(textSpans, TextSpan{
				Start: start,
				End:   end,
				Text:  val,
			})
		}
	}

	// 3) Build output with clean separation
	out := Output{
		SourcePath:       srcPath,
		FrontMatter:      cf.FrontMatter,     // FM only
		ContentRaw:       string(cf.Content), // body only
		ContentTok:       bodyTokens,         // tokens from body only
		ContentTextSpans: textSpans,          // only text tokens
	}

	// 4) Write JSON
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", outDir, err)
	}
	base := strings.TrimSuffix(filepath.Base(srcPath), filepath.Ext(srcPath))
	outPath := filepath.Join(outDir, base+".json")

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		log.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		log.Fatalf("write %s: %v", outPath, err)
	}

	fmt.Printf("Wrote %s (%d bytes)\n", outPath, len(data))
}
