package commonprefix

import "testing"

var testCases = []struct {
	in  []string
	out string
}{
	{[]string{"/var/log/foo", "/var/log/foo.1", "/var/log/foo.gz"}, "/var/log/foo"},
	{[]string{"/hello", "/hola"}, "/h"},
	{[]string{}, ""},
	{[]string{"", "/bar"}, ""},
	{[]string{"A", "B"}, ""},
}

func TestCommonPrefix(t *testing.T) {
	for _, tt := range testCases {
		res := CommonPrefix(tt.in)
		if res != tt.out {
			t.Errorf("Got '%s' from %v, expecting '%s'", res, tt.in, tt.out)
		}
	}
}

func BenchmarkCommonPrefix(b *testing.B) {
	for _, tt := range testCases {
		for i := 0; i < b.N; i++ {
			CommonPrefix(tt.in)
		}
	}
}
