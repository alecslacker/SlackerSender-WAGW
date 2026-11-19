package whatsapp

import (
	"testing"

	"go.mau.fi/whatsmeow/types"
)

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
		// JID perangkat: suffix ":device" dibuang, balasan ke AKUN (bug produksi:
		// "155...:23@lid" membuat usync kehabisan waktu).
		{"155357624197197:23@lid", "155357624197197@lid"},
		{"200506739830854:81@lid", "200506739830854@lid"},
		{"6285777778781:5@s.whatsapp.net", "6285777778781@s.whatsapp.net"},
		{"6285777778781.0:5@s.whatsapp.net", "6285777778781@s.whatsapp.net"},
		// ID grup WhatsApp (format Slackersender) dipakai apa adanya.
		{"120363398726060191@g.us", "120363398726060191@g.us"},
		{" 120363398726060191@g.us ", "120363398726060191@g.us"},
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

func TestSenderJIDTanpaSuffixPerangkat(t *testing.T) {
	// Pengirim berupa JID perangkat: balasan otomatis harus ke akunnya.
	dev := types.JID{User: "155357624197197", Server: types.HiddenUserServer, Device: 23}
	if got := dev.String(); got != "155357624197197:23@lid" {
		t.Fatalf("prasyarat tes: String() = %q", got)
	}
	if got := dev.ToNonAD().String(); got != "155357624197197@lid" {
		t.Errorf("ToNonAD().String() = %q, mau 155357624197197@lid", got)
	}
	jid, err := parseJID(dev.String())
	if err != nil || jid.String() != "155357624197197@lid" {
		t.Errorf("parseJID(JID perangkat) = %q, err=%v", jid.String(), err)
	}
}
