package whatsapp

import "testing"

func TestParseJIDNormalisasiNomor(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"628123456789", "628123456789@s.whatsapp.net"},
		{"+628123456789", "628123456789@s.whatsapp.net"},
		{"08123456789", "628123456789@s.whatsapp.net"},
		{"0812-3456 789", "628123456789@s.whatsapp.net"},
		{" 08888001337 ", "628888001337@s.whatsapp.net"},
		// JID lengkap dipakai apa adanya (balasan ke pengirim LID).
		{"123456@lid", "123456@lid"},
		{"628xx@s.whatsapp.net", "628xx@s.whatsapp.net"},
	}
	for _, tc := range cases {
		got, err := parseJID(tc.in)
		if err != nil {
			t.Errorf("parseJID(%q): error tak terduga: %v", tc.in, err)
			continue
		}
		if got.String() != tc.want {
			t.Errorf("parseJID(%q) = %q, mau %q", tc.in, got.String(), tc.want)
		}
	}
}

func TestParseJIDMenolakKosong(t *testing.T) {
	for _, in := range []string{"", "   ", "+", "0", "@lid", "123@"} {
		if _, err := parseJID(in); err == nil {
			t.Errorf("parseJID(%q) harus error", in)
		}
	}
}
