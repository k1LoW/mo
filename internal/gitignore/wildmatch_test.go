package gitignore

import "testing"

// Cases are taken from git's t/t3070-wildmatch.sh.
func TestWildmatch(t *testing.T) {
	tests := []struct {
		pattern, text string
		flags         int
		want          bool
	}{
		{"foo", "foo", wmPathname, true},
		{"bar", "foo", wmPathname, false},
		{"", "", wmPathname, true},
		{"???", "foo", wmPathname, true},
		{"??", "foo", wmPathname, false},
		{"*", "foo", wmPathname, true},
		{"f*", "foo", wmPathname, true},
		{"*f", "foo", wmPathname, false},
		{"*foo*", "foo", wmPathname, true},
		{"*ob*a*r*", "foobar", wmPathname, true},
		{"*ab", "aaaaaaabababab", wmPathname, true},
		{`foo\*`, "foo*", wmPathname, true},
		{`foo\*bar`, "foobar", wmPathname, false},
		{`f\\oo`, `f\oo`, wmPathname, true},
		{"*[al]?", "ball", wmPathname, true},
		{"[ten]", "ten", wmPathname, false},
		{"**[!te]", "ten", wmPathname, true},
		{"**[!ten]", "ten", wmPathname, false},
		{"t[a-g]n", "ten", wmPathname, true},
		{"t[!a-g]n", "ten", wmPathname, false},
		{"t[!a-g]n", "ton", wmPathname, true},
		{"t[^a-g]n", "ton", wmPathname, true},
		{"a[]]b", "a]b", wmPathname, true},
		{"a[]-]b", "a-b", wmPathname, true},
		{"a[]-]b", "a]b", wmPathname, true},
		{"a[]-]b", "aab", wmPathname, false},
		{"a[]a-]b", "aab", wmPathname, true},
		{"]", "]", wmPathname, true},
		{"foo*bar", "foo/baz/bar", wmPathname, false},
		{"foo*bar", "foo/baz/bar", 0, true},
		{"foo**bar", "foo/baz/bar", wmPathname, false},
		{"foo/**/bar", "foo/bar", wmPathname, true},
		{"foo/**/bar", "foo/baz/bar", wmPathname, true},
		{"foo/**/bar", "foo/b/a/z/bar", wmPathname, true},
		{"**/foo", "foo", wmPathname, true},
		{"**/foo", "XXX/foo", wmPathname, true},
		{"**/foo", "bar/baz/foo", wmPathname, true},
		{"*/foo", "bar/baz/foo", wmPathname, false},
		{"**/bar*", "foo/bar/baz", wmPathname, false},
		{"**/bar/*", "deep/foo/bar/baz", wmPathname, true},
		{"**/bar/*", "deep/foo/bar/baz/", wmPathname, false},
		{"**/bar/**", "deep/foo/bar/baz/", wmPathname, true},
		{"**/bar/*", "deep/foo/bar", wmPathname, false},
		{"**/bar/**", "deep/foo/bar/", wmPathname, true},
		{"**/bar**", "foo/bar/baz", wmPathname, false},
		{"*/bar/**", "foo/bar/baz/x", wmPathname, true},
		{"*/bar/**", "deep/foo/bar/baz/x", wmPathname, false},
		{"**/bar/*/*", "deep/foo/bar/baz/x", wmPathname, true},
		{"foo?bar", "foo/bar", wmPathname, false},
		{"foo[/]bar", "foo/bar", wmPathname, false},
		{"foo[^a-z]bar", "foo/bar", wmPathname, false},
		{"f[^eiu][^eiu][^eiu][^eiu][^eiu]r", "foo-bar", wmPathname, true},
		{"**/t[o]", "foo/bar/baz/to", wmPathname, true},
		{"[[:alpha:]][[:digit:]][[:upper:]]", "a1B", wmPathname, true},
		{"[[:digit:][:upper:][:space:]]", "a", wmPathname, false},
		{"[[:digit:][:upper:][:space:]]", "A", wmPathname, true},
		{"[a-c[:digit:]x-z]", "5", wmPathname, true},
		{"[a-c[:digit:]x-z]", "b", wmPathname, true},
		{"[a-c[:digit:]x-z]", "q", wmPathname, false},
		{"-*-*-*-*-*-*-12-*-*-*-m-*-*-*", "-adobe-courier-bold-o-normal--12-120-75-75-m-70-iso8859-1", wmPathname, true},
		{"XXX/*/*/*/*/*/*/12/*/*/*/m/*/*/*", "XXX/adobe/courier/bold/o/normal//12/120/75/75/m/70/iso8859/1", wmPathname, true},
		{"XXX/*/*/*/*/*/*/12/*/*/*/m/*/*/*", "XXX/adobe/courier/bold/o/normal//12/120/75/75/X/70/iso8859/1", wmPathname, false},
		{"**/*a*b*g*n*t", "abcd/abcdefg/abcdefghijk/abcdefghijklmnop.txt", wmPathname, true},
		{"**/*a*b*g*n*t", "abcd/abcdefg/abcdefghijk/abcdefghijklmnop.txtz", wmPathname, false},
		{"*/*/*", "foo/bba/arr", wmPathname, true},
		{"*/*/*", "foo/bb/aa/rr", wmPathname, false},
		{"**/**/**", "foo/bb/aa/rr", wmPathname, true},
		{"*X*i", "abcXdefXghi", wmPathname, true},
		{"*/*X*/*/*i", "ab/cXd/efXg/hi", wmPathname, true},
		{"**/*X*/**/*i", "ab/cXd/efXg/hi", wmPathname, true},
		{"{draft,private}.md", "draft.md", wmPathname, false},
		{"{draft,private}.md", "{draft,private}.md", wmPathname, true},
		{"[A-Z]oo", "foo", wmPathname | wmCaseFold, true},
		{"FOO", "foo", wmPathname | wmCaseFold, true},
		{"FOO", "foo", wmPathname, false},
	}
	for _, tt := range tests {
		if got := wildmatch(tt.pattern, tt.text, tt.flags); got != tt.want {
			t.Errorf("wildmatch(%q, %q, %d) = %v, want %v", tt.pattern, tt.text, tt.flags, got, tt.want)
		}
	}
}
