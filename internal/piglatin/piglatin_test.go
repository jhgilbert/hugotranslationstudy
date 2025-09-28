package piglatin

import "testing"

func TestPigWord(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		// Vowel-start
		{"apple", "apple", "appleway"},
		{"Apple capitalized", "Apple", "Appleway"},

		// Single consonant
		{"dog", "dog", "ogday"},
		{"Dog capitalized", "Dog", "Ogday"},

		// Consonant cluster
		{"string", "string", "ingstray"},
		{"Smile capitalized", "Smile", "Ilesmay"},

		// No vowels (edge case)
		{"rhythms", "rhythms", "rhythmsay"},
		{"", "", ""}, // empty string
	}

	for _, tc := range tests {
		tc := tc // capture
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := pigWord(tc.in)
			if got != tc.want {
				t.Fatalf("pigWord(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}