package tomarkdoc

import (
	"strings"
	"testing"
)

// Table-driven unit tests for focused cases
func TestConvertBodyToMdocTokens_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "standalone angle",
			in:   `Before {{< note "Remember" >}} After`,
			want: `Before {% note "Remember" /%} After`,
		},
		{
			name: "standalone percent",
			in:   `X {{% badge text="NEW" %}} Y`,
			want: `X {% badge text="NEW" /%} Y`,
		},
		{
			name: "paired angle simple",
			in:   "{{< box title=\"T\" >}}Body{{< /box >}}",
			want: "{% box title=\"T\" %}Body{% /box %}",
		},
		{
			name: "paired percent simple",
			in:   "{{% admonition type=\"tip\" %}}Body{{% /admonition %}}",
			want: "{% admonition type=\"tip\" %}Body{% /admonition %}",
		},
		{
			name: "odd whitespace in closing tag",
			in:   "{{< wrapper >}}X{{<   /   wrapper   >}}",
			want: "{% wrapper %}X{% /wrapper %}",
		},
		{
			name: "inline shortcode in sentence",
			in:   `Hello {{< badge text="HOT" >}} world`,
			want: `Hello {% badge text="HOT" /%} world`,
		},
		{
			name: "back-to-back shortcodes",
			in:   `{{< badge text="ONE" >}}{{< badge text="TWO" >}}`,
			want: `{% badge text="ONE" /%}{% badge text="TWO" /%}`,
		},
		{
			name: "no shortcode (pass-through)",
			in:   "Just text **and** _markdown_.",
			want: "Just text **and** _markdown_.",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ConvertBodyToMdocTokens(tc.in)
			if got != tc.want {
				t.Fatalf("\nConvertBodyToMdocTokens(%q)\n  got : %q\n  want: %q", tc.in, got, tc.want)
			}
		})
	}
}

// A “kitchen-sink” golden test that exercises nesting, percent/angle, inline, list contexts, etc.
func TestConvertBodyToMdocTokens_ComplexGolden(t *testing.T) {
	t.Parallel()

	input := strings.TrimLeft(`
This document covers many shortcode scenarios.

Standalone:
{{< note "Remember to drink water" >}}

Paired angle:
{{< box title="Important Box" >}}
Inside the box.
{{< /box >}}

Nested tabs:
{{< tabs >}}
  {{< tab name="First" >}}A{{< /tab >}}
  {{< tab name="Second" >}}B{{< /tab >}}
{{< /tabs >}}

Percent paired with markdown body:
{{% admonition type="tip" %}}
Tip body with **markdown** and an inline angle {{< badge text="BETA" >}} inside.
{{% /admonition %}}

Oddly spaced closing:
{{< wrapper >}}wrapped{{<   /   wrapper   >}}

Inline and back-to-back:
Before {{< badge text="ONE" >}}{{< badge text="TWO" >}} after.

List context:
- First
- Second with inline {{< badge text="LIST" >}} badge.

Done.
`, "\n")

	// Expected output mirrors input text, but with .mdoc punctuation:
	want := strings.TrimLeft(`
This document covers many shortcode scenarios.

Standalone:
{% note "Remember to drink water" /%}

Paired angle:
{% box title="Important Box" %}
Inside the box.
{% /box %}

Nested tabs:
{% tabs %}
  {% tab name="First" %}A{% /tab %}
  {% tab name="Second" %}B{% /tab %}
{% /tabs %}

Percent paired with markdown body:
{% admonition type="tip" %}
Tip body with **markdown** and an inline angle {% badge text="BETA" /%} inside.
{% /admonition %}

Oddly spaced closing:
{% wrapper %}wrapped{% /wrapper %}

Inline and back-to-back:
Before {% badge text="ONE" /%}{% badge text="TWO" /%} after.

List context:
- First
- Second with inline {% badge text="LIST" /%} badge.

Done.
`, "\n")

	got := ConvertBodyToMdocTokens(input)
	if got != want {
		t.Fatalf("complex conversion mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
