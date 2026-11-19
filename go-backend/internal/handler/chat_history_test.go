package handler

import (
	"reflect"
	"sort"
	"testing"
)

func TestNormalizePhoneVariants(t *testing.T) {
	cases := []struct {
		nama string
		in   string
		want []string
	}{
		{"format lokal 08", "08888001337", []string{"08888001337", "628888001337", "8888001337"}},
		{"format 62", "628888001337", []string{"628888001337", "08888001337", "8888001337"}},
		{"dengan tanda +, spasi, strip", "+62 888-8001-337", []string{"628888001337", "08888001337", "8888001337"}},
		{"tanpa awalan", "8888001337", []string{"8888001337"}},
		{"LID (bukan nomor Indonesia)", "155357624197197", []string{"155357624197197"}},
		{"hanya 0", "0", []string{"0"}},
		{"hanya 62", "62", []string{"62"}},
		{"bukan angka", "abc", nil},
		{"kosong", "", nil},
	}
	for _, tc := range cases {
		got := normalizePhoneVariants(tc.in)
		g := append([]string(nil), got...)
		w := append([]string(nil), tc.want...)
		sort.Strings(g)
		sort.Strings(w)
		if !reflect.DeepEqual(g, w) {
			t.Errorf("%s: normalizePhoneVariants(%q) = %v, mau %v", tc.nama, tc.in, got, tc.want)
		}
	}
}

func TestNormalizePhoneVariantsTanpaDuplikat(t *testing.T) {
	for _, in := range []string{"08123456789", "628123456789", "8123456789"} {
		seen := map[string]bool{}
		for _, v := range normalizePhoneVariants(in) {
			if seen[v] {
				t.Errorf("%q menghasilkan varian ganda %q", in, v)
			}
			seen[v] = true
		}
	}
}
