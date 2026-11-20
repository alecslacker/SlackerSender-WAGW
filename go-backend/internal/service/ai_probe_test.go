package service

import (
	"bytes"
	"errors"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeImagePNGDuaPitaWarna(t *testing.T) {
	top, bottom := probeColors[0].RGBA, probeColors[1].RGBA
	data, err := probeImagePNG(top, bottom)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("PNG tidak valid: %v", err)
	}
	if b := img.Bounds(); b.Dx() != probeImageSize || b.Dy() != probeImageSize {
		t.Errorf("ukuran salah: %v", b)
	}
	at := func(x, y int) color.RGBA {
		r, g, b, a := img.At(x, y).RGBA()
		return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
	}
	if at(5, 5) != top || at(120, 60) != top {
		t.Errorf("separuh atas harus %v", top)
	}
	if at(5, 70) != bottom || at(120, 125) != bottom {
		t.Errorf("separuh bawah harus %v", bottom)
	}
}

func TestParseProbeAnswer(t *testing.T) {
	cases := []struct {
		nama, in, top, bottom string
	}{
		{"format tetap", "ATAS=merah, BAWAH=biru", "merah", "biru"},
		{"huruf kecil spasi", "atas = hijau , bawah = kuning", "hijau", "kuning"},
		{"kalimat Indonesia", "Bagian atas berwarna merah dan bagian bawah berwarna biru.", "merah", "biru"},
		{"Inggris", "The top is red and the bottom is yellow.", "merah", "kuning"},
		{"urutan terbalik", "Bawah: hijau. Atas: biru.", "biru", "hijau"},
		{"baris baru", "ATAS=merah\nBAWAH=hijau", "merah", "hijau"},
		{"tanpa label", "merah dan biru", "", ""},
		{"label tanpa warna", "ATAS=? BAWAH=?", "", ""},
		{"kosong", "", "", ""},
		{"hanya satu", "ATAS=merah", "merah", ""},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			top, bottom := parseProbeAnswer(c.in)
			if top != c.top || bottom != c.bottom {
				t.Errorf("parse(%q) = (%q,%q), mau (%q,%q)", c.in, top, bottom, c.top, c.bottom)
			}
		})
	}
}

func TestProbeVisionAnswer(t *testing.T) {
	cases := []struct {
		nama, ans, status string
	}{
		{"benar keduanya", "ATAS=merah, BAWAH=biru", "yes"},
		{"posisi tertukar", "ATAS=biru, BAWAH=merah", "no"},
		{"satu benar satu salah", "ATAS=merah, BAWAH=hijau", "no"},
		{"mengaku tak bisa melihat", "Maaf, saya tidak bisa melihat gambar.", "no"},
		{"menebak dua warna salah", "ATAS=hijau, BAWAH=kuning", "no"},
		{"kosong -> belum bisa disimpulkan", "   ", ProbeUnknown},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			if got := probeVisionAnswer(c.ans, "merah", "biru"); got.Status != c.status {
				t.Errorf("status = %q, mau %q (%s)", got.Status, c.status, got.Detail)
			}
		})
	}
}

func TestClassifyProbeError(t *testing.T) {
	// penolakan YANG menyebut gambar/vision -> model memang tak menerima gambar
	no := []string{
		"AI error 400: model does not support image input", "Anthropic error 400: invalid media_type",
		"Gemini error 400: Image input modality is not enabled for models/x", "AI error 415: unsupported content type",
		"AI error 422: image_url is not allowed", "AI error 400: this model is text-only, no vision",
	}
	unk := []string{
		// 400 tanpa petunjuk gambar: kunci salah / saldo habis / nama model salah / parameter ditolak
		"AI error 400: bad", "Anthropic error 400: Your credit balance is too low", "Gemini error 400: API key not valid",
		"AI error 400: Unsupported parameter: max_tokens", "AI error 422: z",
		"AI request failed: dial tcp: timeout", "AI error 401: bad key", "AI error 403: nope",
		"AI error 404: model", "AI error 429: rate", "AI error 500: oops", "AI error 503: down",
		`AI error 429: {"detail":"upstream error 400: image x"}`, // body menyebut 400+image tetapi status 429
	}
	for _, s := range no {
		if r := classifyProbeError(errors.New(s)); r.Status != "no" {
			t.Errorf("%q harus 'no', dapat %q", s, r.Status)
		}
	}
	for _, s := range unk {
		if r := classifyProbeError(errors.New(s)); r.Status != ProbeUnknown {
			t.Errorf("%q harus unknown (tak boleh menandai 'tidak mendukung'), dapat %q", s, r.Status)
		}
	}
}

// pickTetap: indeks acak deterministik agar tes bisa memastikan warna yang benar.
// ti=0 (merah) atas; bi=0 -> geser jadi 1 (biru) bawah.
func pickTetap(n int) int { return 0 }

func serverProbe(t *testing.T, balas func(body string) (int, string)) (*httptest.Server, *[]string) {
	t.Helper()
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		code, out := balas(string(b))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(out))
	}))
	t.Cleanup(srv.Close)
	return srv, &bodies
}

func TestProbeVisionModelYangMelihat(t *testing.T) {
	srv, bodies := serverProbe(t, func(string) (int, string) {
		return 200, `{"choices":[{"message":{"content":"ATAS=merah, BAWAH=biru"}}]}`
	})
	s := &AIService{Provider: ProviderCustom, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	r := s.ProbeVision(ProviderCustom, "m", pickTetap)
	if r.Status != "yes" {
		t.Fatalf("harus yes: %+v", r)
	}
	// gambar benar-benar terkirim sebagai image_url data URI PNG
	// (JSON meng-escape "<" ">" jadi \u003c \u003e, jadi periksa potongan polos.)
	if len(*bodies) != 1 || !strings.Contains((*bodies)[0], "data:image/png;base64,") ||
		!strings.Contains((*bodies)[0], "Jawab PERSIS dengan format: ATAS=") {
		t.Errorf("request tak membawa gambar/pertanyaan: %.200s", (*bodies)[0])
	}
}

func TestProbeVisionProxyYangMembuangGambar(t *testing.T) {
	// Proxy menjawab 200 tetapi model tak melihat gambar -> jawaban menebak.
	srv, _ := serverProbe(t, func(string) (int, string) {
		return 200, `{"choices":[{"message":{"content":"ATAS=hijau, BAWAH=kuning"}}]}`
	})
	s := &AIService{Provider: ProviderCustom, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	if r := s.ProbeVision(ProviderCustom, "m", pickTetap); r.Status != "no" {
		t.Errorf("tebakan salah harus no, dapat %+v", r)
	}
}

func TestProbeVisionModelTeksMenolak400(t *testing.T) {
	srv, _ := serverProbe(t, func(string) (int, string) {
		return 400, `{"error":"model does not support image input"}`
	})
	s := &AIService{Provider: ProviderCustom, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	if r := s.ProbeVision(ProviderCustom, "m", pickTetap); r.Status != "no" {
		t.Errorf("400 harus no, dapat %+v", r)
	}
}

func TestProbeVision400TanpaPetunjukGambarUnknown(t *testing.T) {
	for _, body := range []string{`{"error":"invalid api key"}`, `{"error":"credit balance too low"}`, `{"error":"unknown model"}`} {
		srv, _ := serverProbe(t, func(string) (int, string) { return 400, body })
		s := &AIService{Provider: ProviderCustom, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
		if r := s.ProbeVision(ProviderCustom, "m", pickTetap); r.Status != ProbeUnknown {
			t.Errorf("400 %s harus unknown (bukan bukti tak bisa melihat gambar), dapat %+v", body, r)
		}
	}
}

func TestProbeVisionJawabanBerteleteleUnknownBukanNo(t *testing.T) {
	// Model mengulang soal lalu menjawab dengan format tak terduga: belum cukup bukti.
	for _, ans := range []string{
		"Gambar ini memuat separuh atas dan separuh bawah dengan warna berbeda.",
		"Tentu! Berikut analisis saya mengenai pita warna tersebut...",
		"ATAS=merah",
	} {
		if r := probeVisionAnswer(ans, "merah", "biru"); r.Status != ProbeUnknown {
			t.Errorf("%q harus unknown, dapat %q", ans, r.Status)
		}
	}
}

func TestProbeVisionPengakuanTakBisaMelihatAdalahNo(t *testing.T) {
	for _, ans := range []string{
		"Maaf, saya tidak bisa melihat gambar.", "I cannot see any image in this conversation.",
		"I'm sorry, but I can't view images.", "Saya tidak dapat membaca gambar.",
	} {
		if r := probeVisionAnswer(ans, "merah", "biru"); r.Status != "no" {
			t.Errorf("%q harus no, dapat %q", ans, r.Status)
		}
	}
}

func TestProbeVisionGalatSementaraTidakMenandaiNo(t *testing.T) {
	for _, code := range []int{401, 403, 404, 429, 500, 503} {
		code := code
		srv, _ := serverProbe(t, func(string) (int, string) { return code, `{"error":"x"}` })
		s := &AIService{Provider: ProviderCustom, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
		if r := s.ProbeVision(ProviderCustom, "m", pickTetap); r.Status != ProbeUnknown {
			t.Errorf("HTTP %d harus unknown, dapat %+v", code, r)
		}
	}
}

func TestProbeVisionJawabanKosongUnknown(t *testing.T) {
	// Model reasoning kehabisan token -> content kosong -> BUKAN "tidak mendukung".
	srv, _ := serverProbe(t, func(string) (int, string) {
		return 200, `{"choices":[{"message":{"content":"","reasoning_content":"mikir..."},"finish_reason":"length"}]}`
	})
	s := &AIService{Provider: ProviderCustom, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	if r := s.ProbeVision(ProviderCustom, "m", pickTetap); r.Status != ProbeUnknown {
		t.Errorf("kosong harus unknown, dapat %+v", r)
	}
}

// Probe memakai jalur Complete yang sama, jadi harus bekerja lewat Anthropic juga
// (klien Anthropic memakai URL tetap -> RoundTripper tiruan).
func TestProbeVisionLewatAnthropic(t *testing.T) {
	var sent string
	cli := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		sent = string(b)
		return balasHTTP(200, `{"content":[{"type":"text","text":"ATAS=merah, BAWAH=biru"}],"usage":{}}`), nil
	})}
	s := &AIService{Provider: ProviderAnthropic, APIKey: "k", HTTPClient: cli}
	if r := s.ProbeVision(ProviderAnthropic, "claude-x", pickTetap); r.Status != "yes" {
		t.Fatalf("harus yes: %+v", r)
	}
	if !strings.Contains(sent, `"type":"image"`) || !strings.Contains(sent, `"media_type":"image/png"`) {
		t.Errorf("blok gambar Anthropic tak terkirim: %.160s", sent)
	}
}

func TestProbeVisionLewatGemini(t *testing.T) {
	srv, bodies := serverProbe(t, func(string) (int, string) {
		return 200, `{"candidates":[{"content":{"parts":[{"text":"ATAS=merah, BAWAH=biru"}]}}]}`
	})
	s := &AIService{Provider: ProviderGemini, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	if r := s.ProbeVision(ProviderGemini, "gemini-x", pickTetap); r.Status != "yes" {
		t.Fatalf("harus yes: %+v", r)
	}
	if !strings.Contains((*bodies)[0], `"inline_data"`) || !strings.Contains((*bodies)[0], `"mime_type":"image/png"`) {
		t.Errorf("inline_data Gemini tak terkirim: %.160s", (*bodies)[0])
	}
}

func TestProbeVisionDuaWarnaSelaluBerbeda(t *testing.T) {
	for ti := 0; ti < len(probeColors); ti++ {
		for bi := 0; bi < len(probeColors)-1; bi++ {
			calls := 0
			pick := func(n int) int {
				calls++
				if calls == 1 {
					return ti
				}
				return bi
			}
			srv, _ := serverProbe(t, func(string) (int, string) {
				return 200, `{"choices":[{"message":{"content":"x"}}]}`
			})
			s := &AIService{Provider: ProviderCustom, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
			s.ProbeVision(ProviderCustom, "m", pick)
			// bi >= ti digeser: bawah tak pernah sama dengan atas
			b := bi
			if b >= ti {
				b++
			}
			if b == ti {
				t.Errorf("ti=%d bi=%d menghasilkan warna sama", ti, bi)
			}
		}
	}
}

func TestShortErrDipotong(t *testing.T) {
	if got := shortErr("a\n\n b   c"); got != "a b c" {
		t.Errorf("spasi harus dirapatkan: %q", got)
	}
	long := make([]rune, 400)
	for i := range long {
		long[i] = 'x'
	}
	if got := []rune(shortErr(string(long))); len(got) != 161 {
		t.Errorf("harus 160 + elipsis, dapat %d", len(got))
	}
}
