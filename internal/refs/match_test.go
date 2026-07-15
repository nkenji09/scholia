package refs

import (
	"reflect"
	"testing"
)

func TestFindOccurrences(t *testing.T) {
	cases := []struct {
		name    string
		content string
		id      string
		want    []int
	}{
		{
			name:    "clean match surrounded by spaces",
			content: "see req.foo for context",
			id:      "req.foo",
			want:    []int{4},
		},
		{
			name:    "rejects alnum-continued token (req.foobar)",
			content: "see req.foobar over there",
			id:      "req.foo",
			want:    nil,
		},
		{
			name:    "rejects delimiter-then-alnum continued token (req.foo-bar)",
			content: "see req.foo-bar over there",
			id:      "req.foo",
			want:    nil,
		},
		{
			name:    "rejects dot-then-alnum continued token (req.foo.bar)",
			content: "see req.foo.bar over there",
			id:      "req.foo",
			want:    nil,
		},
		{
			name:    "rejects leading continuation (xreq.foo)",
			content: "see xreq.foo over there",
			id:      "req.foo",
			want:    nil,
		},
		{
			name:    "accepts trailing sentence punctuation",
			content: "see req.foo. done",
			id:      "req.foo",
			want:    []int{4},
		},
		{
			name:    "accepts trailing punctuation run with no alnum",
			content: "see req.foo... done",
			id:      "req.foo",
			want:    []int{4},
		},
		{
			name:    "accepts trailing comma-like non-idchar boundary",
			content: "see req.foo, done",
			id:      "req.foo",
			want:    []int{4},
		},
		{
			name:    "accepts at end of file with no trailing char",
			content: "see req.foo",
			id:      "req.foo",
			want:    []int{4},
		},
		{
			name:    "accepts at start of file",
			content: "req.foo is here",
			id:      "req.foo",
			want:    []int{0},
		},
		{
			name:    "does not match unrelated text",
			content: "nothing to see here",
			id:      "req.foo",
			want:    nil,
		},
		{
			name:    "finds multiple clean occurrences",
			content: "req.foo appears twice: req.foo",
			id:      "req.foo",
			want:    []int{0, 23},
		},
		{
			name:    "ignores string literal occurrence (still a clean boundary match)",
			content: `x := "req.foo"`,
			id:      "req.foo",
			want:    []int{6},
		},
		{
			name:    "sibling id with shared prefix stays untouched (cascade shape)",
			content: "req.atoms-derive.no-spec-file is unrelated to req.atoms-derive",
			id:      "req.atoms-derive",
			want:    []int{46},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := findOccurrences([]byte(c.content), c.id)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("findOccurrences(%q, %q) = %v, want %v", c.content, c.id, got, c.want)
			}
		})
	}
}
