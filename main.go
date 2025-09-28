package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/gohugoio/hugo/parser/pageparser"
)

func main() {
	// 1) Read a Hugo content file (Markdown)
	b, err := os.ReadFile("content/example.md")
	if err != nil {
		log.Fatal(err)
	}

	// 2) Quickly split front matter vs. content
	cf, err := pageparser.ParseFrontMatterAndContent(bytes.NewReader(b))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== FRONT MATTER (decoded) ===")
	fmJSON, _ := json.MarshalIndent(cf.FrontMatter, "", "  ")
	fmt.Println(string(fmJSON))

	fmt.Println("\n=== CONTENT (raw Markdown body) ===")
	fmt.Println(string(cf.Content))

	// 3) Tokenize the whole file (front matter + body) to see what pageparser recognizes
	fmt.Println("\n=== TOKENS (type, value) ===")
	res, err := pageparser.ParseMain(bytes.NewReader(b), pageparser.Config{})
	if err != nil {
		log.Fatal(err)
	}
	it := res.Iterator()
	src := res.Input()

	for {
		item := it.Next()
		// End-of-input or error
		if item.IsEOF() || item.IsDone() {
			break
		}
		// Show a compact description of each item.
		typ := item.Type.String()
		val := item.ValStr(src)
		// Keep lines short: trim newlines and long text
		if len(val) > 80 {
			val = val[:77] + "..."
		}
		fmt.Printf("%-28s | %q\n", typ, val)
	}
}
