# Deep tokenization feature

The goal of our file tokenization is to isolate translatable text while protecting anything that should not be translated, such as a shortcode name (like `tabs`).

The problem with the current tokenization strategy is that the resulting tokens are not always pure translatable text. For example, a given content token in `content/03_fences_and_html.md` can still contain raw HTML, code fences, and so on. 

We need a more granular breakdown of a token like this one:

```
[
    {
      "type": "tText",
      "val": "\n## Overview\n\nThis file contains \u003cspan id=\"some-span\"\u003eraw html\u003c/span\u003e and some code fences. There is also **bold text**.\n\n```javascript\nconsole.log(\"Hello world\")\n```\n\n\u003cdiv class=\"alert alert-info\"\u003eWhen in doubt, just ask \u003ca href=\"https://www.google.com\"\u003eGoogle\u003c/a\u003e!\u003cdiv\u003e"
    }
  ]
```

The breakdown should protect anything that should not be translated, such as HTML tag names, code, and so on. Items like "**bold text**" should also be protected, with the translator just getting a "bold text" string. Same for links -- anything with translatable text in it (in the case of links, the link description) should be parsed such that the translatable text is fully isolated.

Frontmatter can be left as-is for now.

It's preferred to use existing parsers rather than writing our own. For example, we might use a "Markdoc to HTML" parser that supports Goldmark, and then an "HTML to JSON" parser that further breaks down the HTML into pieces. If the Goldmark parser alone can handle all of our needs, that's fine too -- the end result is what matters.

Keep in mind that this process must be reversible: after translation, we must be able to reassemble the markup as it was before, weaving everything back together.

The fine-grained tokens should appear in a `subtokens` field added to the `tText` token object.

## Claude's plan

### Approach: Goldmark AST gap-filling

Parse each `tText` value with Goldmark to get an AST. Walk the AST and collect byte ranges for all **leaf content** (text nodes, code content, raw HTML segments). Classify each range as either translatable or protected based on context. Then use **gap-filling** to turn the unclaimed bytes (markdown syntax like `##`, `**`, `` ``` ``, `[](url)`, etc.) into protected markup subtokens. Finally, for `HTMLBlock` nodes (block-level raw HTML), sub-parse with `golang.org/x/net/html`'s tokenizer to separate HTML tags (protected) from text content (translatable).

This approach works because Goldmark's AST nodes reference exact byte offsets in the source. Any source bytes not covered by an AST leaf node are structural markdown syntax — exactly what we want to protect. And since we build subtokens directly from source byte ranges, concatenating all subtoken values always reproduces the original input exactly, guaranteeing reversibility.

The `golang.org/x/net/html` package is already an indirect dependency (via Hugo), so no new dependencies are needed.

**Known limitation:** When Hugo's pageparser splits a `tText` token mid-table (at a shortcode boundary), each fragment is parsed by Goldmark independently and won't be recognized as table syntax. Table cell markers (`|`) would end up in text subtokens. This is acceptable for now — it's a pre-existing pageparser limitation, and table markup doesn't need the same protection as code or HTML tags.

---

### Step 1: Define the `Subtoken` type

Add to `main.go` (alongside the existing `Token` type):

```go
type Subtoken struct {
    Type string `json:"type"` // "text" or "markup"
    Val  string `json:"val"`
}
```

- `"text"` — translatable content (pure human-readable text)
- `"markup"` — protected content (do not translate: markdown syntax, HTML tags, code, etc.)

Add a `Subtokens` field to the existing `Token` struct:

```go
type Token struct {
    Type      string     `json:"type"`
    Val       string     `json:"val"`
    Subtokens []Subtoken `json:"subtokens,omitempty"`
}
```

---

### Step 2: Create `internal/subtokenize/subtokenize.go`

New package with two exported items:

```go
package subtokenize

type Subtoken struct {
    Type string `json:"type"`
    Val  string `json:"val"`
}

func Subtokenize(source []byte) ([]Subtoken, error)
```

#### `Subtokenize` algorithm

1. **Parse** the source bytes with Goldmark (with the Table extension enabled, to match Hugo's defaults).
2. **Walk** the AST depth-first, collecting **claimed ranges** — each is a `(start, stop, type)` tuple:
   - `*ast.Text` nodes: collect `Segment` as `"text"`, UNLESS the node is inside a `*ast.CodeSpan` or `*ast.AutoLink` ancestor, in which case collect as `"markup"`.
   - `*ast.RawHTML` nodes (inline HTML tags like `<span>`): collect each segment as `"markup"`.
   - `*ast.FencedCodeBlock` / `*ast.CodeBlock`: collect each line from `Lines()` as `"markup"`.
   - `*ast.HTMLBlock`: collect all lines from `Lines()`, then **sub-parse** the concatenated bytes with `subtokenizeHTML()` (Step 3). The sub-parsed subtokens replace the raw lines in the output (with byte offsets adjusted to the block's position in the source).
3. **Sort** all claimed ranges by start position.
4. **Fill gaps**: walk the source from byte 0 to end. For each gap (bytes between claimed ranges), emit a `"markup"` subtoken. For each claimed range, emit it with its type.
5. **Merge** adjacent subtokens of the same type for cleaner output.
6. Return the subtoken list.

The context tracking (for CodeSpan/AutoLink ancestors) uses a simple stack or boolean flags toggled on enter/exit during the walk:

```go
type walker struct {
    source       []byte
    ranges       []claimedRange
    inCodeSpan   bool
    inAutoLink   bool
}
```

---

### Step 3: HTML sub-parser — `subtokenizeHTML()`

Unexported helper in the same package:

```go
func subtokenizeHTML(source []byte) []Subtoken
```

Uses `golang.org/x/net/html.NewTokenizer` to walk the HTML bytes. For each token:
- `html.StartTagToken`, `html.EndTagToken`, `html.SelfClosingTagToken`, `html.CommentToken`, `html.DoctypeToken` → `"markup"`
- `html.TextToken` → `"text"`

Uses `tokenizer.Raw()` for each token to get the exact source bytes (preserving original formatting, attributes, whitespace). This guarantees reversibility.

**Example input/output for `subtokenizeHTML`:**

Input:
```
<div class="alert alert-info">When in doubt, just ask <a href="https://www.google.com">Google</a>!<div>
```

Output:
```json
[
  {"type": "markup", "val": "<div class=\"alert alert-info\">"},
  {"type": "text",   "val": "When in doubt, just ask "},
  {"type": "markup", "val": "<a href=\"https://www.google.com\">"},
  {"type": "text",   "val": "Google"},
  {"type": "markup", "val": "</a>"},
  {"type": "text",   "val": "!"},
  {"type": "markup", "val": "<div>"}
]
```

---

### Step 4: Integrate into `main.go` — populate subtokens during parsing

In `parseAndWriteJSON`, after building each `Token` with type `"tText"`, call `subtokenize.Subtokenize()` and attach the result:

```go
if tok.Type == "tText" && len(valB) > 0 {
    subs, err := subtokenize.Subtokenize(valB)
    if err != nil {
        log.Printf("warning: subtokenize failed: %v", err)
    } else {
        tok.Subtokens = subs
    }
}
```

The `Subtoken` type in `main.go` mirrors `subtokenize.Subtoken` (or we import it directly — either works). The JSON output gains a `"subtokens"` array on each `tText` token.

---

### Step 5: Update translation to use subtokens

Modify `translateBodyUsingRanges` so that when a tText token has subtokens, only the `"text"` subtokens are translated:

```go
// For each text span (processed in reverse for safe splicing):
//   1. Find the matching tText token (by index — they're 1:1 in order)
//   2. If token has subtokens:
//        - translate only "text" subtokens with piglatin.ToPigLatin()
//        - concatenate all subtokens to produce the translated value
//   3. Else: translate the whole span as before (fallback)
//   4. Splice the translated value into the body at the span's byte range
```

This is backward-compatible: files whose tText tokens have no subtokens (e.g., if subtokenization fails or is skipped) still work via the existing whole-span translation.

---

### Step 6: Tests

Create `internal/subtokenize/subtokenize_test.go`.

#### Test 1: `TestSubtokenize_SimpleEmphasis`

Input: `"Hello **world**!\n"`

Expected:
```json
[
  {"type": "text",   "val": "Hello "},
  {"type": "markup", "val": "**"},
  {"type": "text",   "val": "world"},
  {"type": "markup", "val": "**"},
  {"type": "text",   "val": "!"},
  {"type": "markup", "val": "\n"}
]
```

Verifies: emphasis delimiters are protected, text content is translatable.

#### Test 2: `TestSubtokenize_CodeFence`

Input:
```
Some text.

` `` `javascript
console.log("Hello")
` `` `

More text.

```
(Without the spaces in the backtick fences — escaping for readability.)

Expected: the paragraph text ("Some text." and "More text.") are `"text"` subtokens; the entire code fence block (opening fence, code content, closing fence, and surrounding blank lines) is one merged `"markup"` subtoken.

#### Test 3: `TestSubtokenize_InlineHTML`

Input: `"Click <a href=\"https://example.com\">here</a> now.\n"`

Expected:
```json
[
  {"type": "text",   "val": "Click "},
  {"type": "markup", "val": "<a href=\"https://example.com\">"},
  {"type": "text",   "val": "here"},
  {"type": "markup", "val": "</a>"},
  {"type": "text",   "val": " now."},
  {"type": "markup", "val": "\n"}
]
```

Verifies: inline HTML tags are protected, text between tags is translatable.

#### Test 4: `TestSubtokenize_HTMLBlock`

Input: `"<div class=\"box\">Hello <b>world</b></div>\n"`

Expected:
```json
[
  {"type": "markup", "val": "<div class=\"box\">"},
  {"type": "text",   "val": "Hello "},
  {"type": "markup", "val": "<b>"},
  {"type": "text",   "val": "world"},
  {"type": "markup", "val": "</b>"},
  {"type": "markup", "val": "</div>\n"}
]
```

Verifies: HTML block triggers sub-parsing; tags protected, text translatable.

#### Test 5: `TestSubtokenize_Link`

Input: `"See the [documentation](https://example.com) for details.\n"`

Expected:
```json
[
  {"type": "text",   "val": "See the "},
  {"type": "markup", "val": "["},
  {"type": "text",   "val": "documentation"},
  {"type": "markup", "val": "](https://example.com)"},
  {"type": "text",   "val": " for details."},
  {"type": "markup", "val": "\n"}
]
```

Verifies: link URL/syntax protected, link text and surrounding text translatable.

#### Test 6: `TestSubtokenize_Heading`

Input: `"## Overview\n"`

Expected:
```json
[
  {"type": "markup", "val": "## "},
  {"type": "text",   "val": "Overview"},
  {"type": "markup", "val": "\n"}
]
```

Verifies: heading markers protected, heading text translatable.

#### Test 7: `TestSubtokenize_InlineCode`

Input: `"Use the ` + "`" + `fmt.Println` + "`" + ` function.\n"`

Expected:
```json
[
  {"type": "text",   "val": "Use the "},
  {"type": "markup", "val": "`fmt.Println`"},
  {"type": "text",   "val": " function."},
  {"type": "markup", "val": "\n"}
]
```

Verifies: inline code (including backtick delimiters) is fully protected.

#### Test 8: `TestSubtokenize_Reversibility`

For each of the above inputs (and the full `03_fences_and_html.md` tText value), verify that concatenating all subtoken `Val` fields reproduces the original input byte-for-byte:

```go
var buf bytes.Buffer
for _, st := range subtokens {
    buf.WriteString(st.Val)
}
assert(buf.String() == string(source))
```

#### Test 9: `TestSubtokenizeHTML`

Directly tests the HTML sub-parser with the input/output shown in Step 3.

---

### Step 7: End-to-end verification

After implementation, run the full pipeline (`go run .`) and verify:

1. `out/03_fences_and_html/data.json` — the tText token now has a `subtokens` array with fine-grained breakdown.
2. `out/03_fences_and_html/translated.md` — HTML tags, code fences, and markdown syntax are preserved as-is; only the human-readable text is pig-latin-ified. Expected output:
   ```
   ## Overviewway

   Isthay ilefay ontainscay <span id="some-span">awray htmlay</span> andway omesay odecay encesfay. Erethay isway alsoway **oldbay exttay**.

   ```javascript
   console.log("Hello world")
   ```

   <div class="alert alert-info">Enwhay inway oubtday, ustjay askway <a href="https://www.google.com">Ooglegay</a>!<div>
   ```
3. `out/01_simple/translated.md` and `out/02_complex/translated.md` — existing behavior preserved or improved.

---

### Summary of files changed

| File | Change |
|------|--------|
| `internal/subtokenize/subtokenize.go` | **New** — `Subtokenize()`, `subtokenizeHTML()`, gap-filling logic |
| `internal/subtokenize/subtokenize_test.go` | **New** — 9 test functions |
| `main.go` | Add `Subtoken` type, add `Subtokens` field to `Token`, call subtokenize in `parseAndWriteJSON`, update `translateBodyUsingRanges` |