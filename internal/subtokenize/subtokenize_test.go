package subtokenize

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// helper: verify reversibility (concatenation reproduces original)
func assertReversible(t *testing.T, source string, subs []Subtoken) {
	t.Helper()
	var buf bytes.Buffer
	for _, s := range subs {
		buf.WriteString(s.Val)
	}
	if buf.String() != source {
		t.Errorf("reversibility failed:\n  source: %q\n  concat: %q", source, buf.String())
	}
}

// helper: compare subtokens to expected
func assertSubtokens(t *testing.T, got []Subtoken, want []Subtoken) {
	t.Helper()
	if len(got) != len(want) {
		gotJSON, _ := json.MarshalIndent(got, "", "  ")
		wantJSON, _ := json.MarshalIndent(want, "", "  ")
		t.Fatalf("length mismatch: got %d, want %d\ngot:\n%s\nwant:\n%s",
			len(got), len(want), gotJSON, wantJSON)
	}
	for i := range want {
		if got[i].Type != want[i].Type || got[i].Val != want[i].Val {
			gotJSON, _ := json.MarshalIndent(got, "", "  ")
			wantJSON, _ := json.MarshalIndent(want, "", "  ")
			t.Fatalf("mismatch at index %d:\n  got:  {%q, %q}\n  want: {%q, %q}\n\nfull got:\n%s\nfull want:\n%s",
				i, got[i].Type, got[i].Val, want[i].Type, want[i].Val, gotJSON, wantJSON)
		}
	}
}

func TestSubtokenize_SimpleEmphasis(t *testing.T) {
	source := "Hello **world**!\n"
	subs, err := Subtokenize([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	assertReversible(t, source, subs)

	want := []Subtoken{
		{Type: "text", Val: "Hello "},
		{Type: "markup", Val: "**"},
		{Type: "text", Val: "world"},
		{Type: "markup", Val: "**"},
		{Type: "text", Val: "!"},
		{Type: "markup", Val: "\n"},
	}
	assertSubtokens(t, subs, want)
}

func TestSubtokenize_CodeFence(t *testing.T) {
	source := "Some text.\n\n```javascript\nconsole.log(\"Hello\")\n```\n\nMore text.\n"
	subs, err := Subtokenize([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	assertReversible(t, source, subs)

	// Verify text subtokens contain paragraph text
	var textVals []string
	for _, s := range subs {
		if s.Type == "text" {
			textVals = append(textVals, s.Val)
		}
	}

	foundSome := false
	foundMore := false
	for _, v := range textVals {
		if strings.Contains(v, "Some text.") {
			foundSome = true
		}
		if strings.Contains(v, "More text.") {
			foundMore = true
		}
	}
	if !foundSome || !foundMore {
		t.Errorf("expected paragraph text in text subtokens, got: %v", textVals)
	}

	// Verify code fence content is NOT in text subtokens
	for _, v := range textVals {
		if strings.Contains(v, "console.log") {
			t.Errorf("code fence content should not be translatable, but found: %q", v)
		}
		if strings.Contains(v, "javascript") {
			t.Errorf("code fence language should not be translatable, but found: %q", v)
		}
	}
}

func TestSubtokenize_InlineHTML(t *testing.T) {
	source := "Click <a href=\"https://example.com\">here</a> now.\n"
	subs, err := Subtokenize([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	assertReversible(t, source, subs)

	want := []Subtoken{
		{Type: "text", Val: "Click "},
		{Type: "markup", Val: "<a href=\"https://example.com\">"},
		{Type: "text", Val: "here"},
		{Type: "markup", Val: "</a>"},
		{Type: "text", Val: " now."},
		{Type: "markup", Val: "\n"},
	}
	assertSubtokens(t, subs, want)
}

func TestSubtokenize_HTMLBlock(t *testing.T) {
	source := "<div class=\"box\">Hello <b>world</b></div>\n"
	subs, err := Subtokenize([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	assertReversible(t, source, subs)

	want := []Subtoken{
		{Type: "markup", Val: "<div class=\"box\">"},
		{Type: "text", Val: "Hello "},
		{Type: "markup", Val: "<b>"},
		{Type: "text", Val: "world"},
		{Type: "markup", Val: "</b></div>"},
		{Type: "text", Val: "\n"},
	}
	assertSubtokens(t, subs, want)
}

func TestSubtokenize_Link(t *testing.T) {
	source := "See the [documentation](https://example.com) for details.\n"
	subs, err := Subtokenize([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	assertReversible(t, source, subs)

	want := []Subtoken{
		{Type: "text", Val: "See the "},
		{Type: "markup", Val: "["},
		{Type: "text", Val: "documentation"},
		{Type: "markup", Val: "](https://example.com)"},
		{Type: "text", Val: " for details."},
		{Type: "markup", Val: "\n"},
	}
	assertSubtokens(t, subs, want)
}

func TestSubtokenize_Heading(t *testing.T) {
	source := "## Overview\n"
	subs, err := Subtokenize([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	assertReversible(t, source, subs)

	want := []Subtoken{
		{Type: "markup", Val: "## "},
		{Type: "text", Val: "Overview"},
		{Type: "markup", Val: "\n"},
	}
	assertSubtokens(t, subs, want)
}

func TestSubtokenize_InlineCode(t *testing.T) {
	source := "Use the `fmt.Println` function.\n"
	subs, err := Subtokenize([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	assertReversible(t, source, subs)

	// Verify inline code is fully markup
	want := []Subtoken{
		{Type: "text", Val: "Use the "},
		{Type: "markup", Val: "`fmt.Println`"},
		{Type: "text", Val: " function."},
		{Type: "markup", Val: "\n"},
	}
	assertSubtokens(t, subs, want)
}

func TestSubtokenize_Reversibility(t *testing.T) {
	// Test reversibility across multiple diverse inputs
	inputs := []string{
		"Hello **world**!\n",
		"Some text.\n\n```javascript\nconsole.log(\"Hello\")\n```\n\nMore text.\n",
		"Click <a href=\"https://example.com\">here</a> now.\n",
		"<div class=\"box\">Hello <b>world</b></div>\n",
		"See the [documentation](https://example.com) for details.\n",
		"## Overview\n",
		"Use the `fmt.Println` function.\n",
		// The full 03_fences_and_html.md tText value
		"\n## Overview\n\nThis file contains <span id=\"some-span\">raw html</span> and some code fences. There is also **bold text**.\n\n```javascript\nconsole.log(\"Hello world\")\n```\n\n<div class=\"alert alert-info\">When in doubt, just ask <a href=\"https://www.google.com\">Google</a>!<div>",
		// Edge cases
		"\n",
		"",
		"Plain text with no markup.\n",
	}

	for _, input := range inputs {
		subs, err := Subtokenize([]byte(input))
		if err != nil {
			t.Errorf("Subtokenize(%q) error: %v", input, err)
			continue
		}
		var buf bytes.Buffer
		for _, s := range subs {
			buf.WriteString(s.Val)
		}
		if buf.String() != input {
			t.Errorf("reversibility failed for input %q:\n  got: %q", input, buf.String())
		}
	}
}

func TestSubtokenizeHTML(t *testing.T) {
	source := `<div class="alert alert-info">When in doubt, just ask <a href="https://www.google.com">Google</a>!<div>`
	subs := subtokenizeHTML([]byte(source))

	// Verify reversibility
	var buf bytes.Buffer
	for _, s := range subs {
		buf.WriteString(s.Val)
	}
	if buf.String() != source {
		t.Errorf("reversibility failed:\n  source: %q\n  concat: %q", source, buf.String())
	}

	want := []Subtoken{
		{Type: "markup", Val: `<div class="alert alert-info">`},
		{Type: "text", Val: "When in doubt, just ask "},
		{Type: "markup", Val: `<a href="https://www.google.com">`},
		{Type: "text", Val: "Google"},
		{Type: "markup", Val: "</a>"},
		{Type: "text", Val: "!"},
		{Type: "markup", Val: "<div>"},
	}
	assertSubtokens(t, subs, want)
}

func TestSubtokenize_FullExample(t *testing.T) {
	// The tText value from 03_fences_and_html.md
	source := "\n## Overview\n\nThis file contains <span id=\"some-span\">raw html</span> and some code fences. There is also **bold text**.\n\n```javascript\nconsole.log(\"Hello world\")\n```\n\n<div class=\"alert alert-info\">When in doubt, just ask <a href=\"https://www.google.com\">Google</a>!<div>"

	subs, err := Subtokenize([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	assertReversible(t, source, subs)

	// Verify key properties rather than exact token list,
	// since Goldmark's exact segment boundaries may vary.

	// 1. "Overview" should be translatable text
	foundOverview := false
	for _, s := range subs {
		if s.Type == "text" && strings.Contains(s.Val, "Overview") {
			foundOverview = true
		}
	}
	if !foundOverview {
		t.Error("expected 'Overview' in a text subtoken")
	}

	// 2. Code fence content should be markup
	for _, s := range subs {
		if s.Type == "text" && strings.Contains(s.Val, "console.log") {
			t.Error("code fence content should not be translatable")
		}
	}

	// 3. HTML tags should be markup
	for _, s := range subs {
		if s.Type == "text" && strings.Contains(s.Val, "<span") {
			t.Error("<span> tag should not be translatable")
		}
		if s.Type == "text" && strings.Contains(s.Val, "<div") {
			t.Error("<div> tag should not be translatable")
		}
		if s.Type == "text" && strings.Contains(s.Val, "<a href") {
			t.Error("<a> tag should not be translatable")
		}
	}

	// 4. "bold text" should be translatable (without ** markers)
	foundBold := false
	for _, s := range subs {
		if s.Type == "text" && strings.Contains(s.Val, "bold text") {
			foundBold = true
			if strings.Contains(s.Val, "**") {
				t.Error("emphasis markers should not be in text subtoken")
			}
		}
	}
	if !foundBold {
		t.Error("expected 'bold text' in a text subtoken")
	}

	// 5. "Google" (link text in HTML block) should be translatable
	foundGoogle := false
	for _, s := range subs {
		if s.Type == "text" && strings.Contains(s.Val, "Google") {
			foundGoogle = true
		}
	}
	if !foundGoogle {
		t.Error("expected 'Google' in a text subtoken")
	}

	// 6. "raw html" (between <span> tags) should be translatable
	foundRawHTML := false
	for _, s := range subs {
		if s.Type == "text" && strings.Contains(s.Val, "raw html") {
			foundRawHTML = true
		}
	}
	if !foundRawHTML {
		t.Error("expected 'raw html' in a text subtoken")
	}
}
