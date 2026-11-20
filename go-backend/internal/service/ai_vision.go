package service

// Dukungan gambar (vision) untuk chat AI.
//
// ChatMessage.Images diisi pemanggil (hanya pada pesan user terakhir); di sini
// tiap provider merangkainya ke format resmi masing-masing:
//   - OpenAI-compatible: content = [{type:text}, {type:image_url, data URI base64}]
//   - Anthropic:         content = [{type:image, source:{base64,media_type}}, {type:text}]
//   - Gemini:            parts   = [{inline_data:{mime_type,data}}, {text}]
// Pesan TANPA gambar dirender persis seperti sebelumnya (content string).
// Teks kosong tidak dikirim sebagai blok — Anthropic menolak blok teks kosong.

import (
	"encoding/base64"
)

// ChatImage adalah satu gambar yang menyertai pesan (bytes mentah + MIME).
type ChatImage struct {
	MIME string
	Data []byte
}

func hasImages(msgs []ChatMessage) bool {
	for _, m := range msgs {
		if len(m.Images) > 0 {
			return true
		}
	}
	return false
}

// OpenAIMessages mengembalikan nilai untuk field "messages" format OpenAI.
// Tanpa gambar: slice asli (JSON identik dengan perilaku lama).
func OpenAIMessages(msgs []ChatMessage) interface{} {
	if !hasImages(msgs) {
		return msgs
	}
	out := make([]map[string]interface{}, 0, len(msgs))
	for _, m := range msgs {
		if len(m.Images) == 0 {
			out = append(out, map[string]interface{}{"role": m.Role, "content": m.Content})
			continue
		}
		parts := make([]interface{}, 0, len(m.Images)+1)
		if m.Content != "" {
			parts = append(parts, map[string]interface{}{"type": "text", "text": m.Content})
		}
		for _, img := range m.Images {
			parts = append(parts, map[string]interface{}{
				"type": "image_url",
				"image_url": map[string]interface{}{
					"url": "data:" + img.MIME + ";base64," + base64.StdEncoding.EncodeToString(img.Data),
				},
			})
		}
		out = append(out, map[string]interface{}{"role": m.Role, "content": parts})
	}
	return out
}

// anthropicMessages memisahkan system prompt dan merangkai pesan format Anthropic.
func anthropicMessages(msgs []ChatMessage) (system string, out []map[string]interface{}) {
	out = []map[string]interface{}{}
	for _, m := range msgs {
		if m.Role == "system" {
			system = m.Content
			continue
		}
		if len(m.Images) == 0 {
			out = append(out, map[string]interface{}{"role": m.Role, "content": m.Content})
			continue
		}
		blocks := make([]interface{}, 0, len(m.Images)+1)
		for _, img := range m.Images {
			blocks = append(blocks, map[string]interface{}{
				"type": "image",
				"source": map[string]interface{}{
					"type":       "base64",
					"media_type": img.MIME,
					"data":       base64.StdEncoding.EncodeToString(img.Data),
				},
			})
		}
		if m.Content != "" {
			blocks = append(blocks, map[string]interface{}{"type": "text", "text": m.Content})
		}
		out = append(out, map[string]interface{}{"role": m.Role, "content": blocks})
	}
	return system, out
}

// geminiParts merangkai parts satu pesan format Gemini (gambar dulu, lalu teks).
func geminiParts(m ChatMessage) []interface{} {
	if len(m.Images) == 0 {
		return []interface{}{map[string]string{"text": m.Content}}
	}
	parts := make([]interface{}, 0, len(m.Images)+1)
	for _, img := range m.Images {
		parts = append(parts, map[string]interface{}{
			"inline_data": map[string]string{
				"mime_type": img.MIME,
				"data":      base64.StdEncoding.EncodeToString(img.Data),
			},
		})
	}
	if m.Content != "" {
		parts = append(parts, map[string]string{"text": m.Content})
	}
	return parts
}
