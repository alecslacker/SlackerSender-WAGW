package whatsapp

// checkAIReply memeriksa config AI auto-reply yang aktif untuk device pengirim
// dan membalas pesan via chatbot AI bila cocok.
//
// Dipanggil via goroutine dari handleIncomingMessage — non-blocking dan gagal
// diam-diam (hanya log), sehingga tidak mengganggu alur pesan yang sudah ada.
//
// Alur chatbot (paket Pro/Master, lihat service.ChatbotAllowed):
//  1. Retrieval FAQ — bila ada entri FAQ/katalog yang cocok, jawab dari entri itu.
//  2. StrictFAQ — bila mode ketat aktif dan tak ada entri cocok, tolak sopan tanpa LLM.
//  3. Fallback LLM — teruskan ke koneksi AI milik user (atau backend global)
//     dengan N pesan terakhir sebagai konteks percakapan.
//  4. Guardrail — jawaban dipotong ke MaxReplyLength.
// Paket lain (Free/Lite/Regular) memakai jalur LLM lama tanpa FAQ/konteks.
// Setiap balasan dicatat ke AIChatbotLog agar bisa diaudit.
//
// Gambar (vision): bila cfg.ReadImages aktif DAN paket Pro/Master, gambar masuk
// diunduh ke memori hanya saat pesan benar-benar menuju LLM (setelah FAQ /
// StrictFAQ), lalu dikirim bersama caption. Gagal baca gambar → degradasi ke
// teks saja, atau pesan sopan bila tak ada caption (lihat ai_image.go).

import (
	"context"
	"strings"
	"time"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/service"
	"github.com/rs/zerolog/log"
	"go.mau.fi/whatsmeow/types/events"
)

// logChatbotReply mencatat satu balasan chatbot (best-effort, abaikan error).
func (m *Manager) logChatbotReply(userID, deviceID uint, phone, question, answer, source string, faqID *uint) {
	if strings.TrimSpace(question) == "" {
		return
	}
	m.db.Create(&models.AIChatbotLog{
		UserID:   userID,
		DeviceID: deviceID,
		Phone:    phone,
		Question: question,
		Answer:   answer,
		Source:   source,
		FAQID:    faqID,
	})
}

// sendChatbotReply mengirim balasan + mencatat log-nya. Mengembalikan true
// bila pesan terkirim.
func (m *Manager) sendChatbotReply(sess *SessionState, senderJID, senderPhone, question, reply, source string, faqID *uint) bool {
	if err := m.SendMessage(sess.DeviceID, senderJID, "text", reply, ""); err != nil {
		log.Error().Err(err).
			Uint("deviceID", sess.DeviceID).
			Str("to", senderJID).
			Str("source", source).
			Msg("AI reply send failed")
		return false
	}
	m.logChatbotReply(sess.UserID, sess.DeviceID, senderPhone, question, reply, source, faqID)
	log.Info().
		Uint("deviceID", sess.DeviceID).
		Str("to", senderJID).
		Str("source", source).
		Msg("AI reply sent")
	return true
}

func (m *Manager) checkAIReply(sess *SessionState, senderJID, senderPhone, text string, isGroup bool, msg *events.Message) {
	hasImage := isImageMessage(msg)
	// Tanpa teks dan bukan gambar: tak ada yang bisa dibalas.
	if strings.TrimSpace(text) == "" && !hasImage {
		return
	}

	var cfgs []models.AIReplyConfig
	m.db.Where("user_id = ? AND device_id = ? AND is_enabled = ?",
		sess.UserID, sess.DeviceID, true).
		Order("id ASC").
		Find(&cfgs)

	for _, cfg := range cfgs {
		if isGroup && cfg.IgnoreGroups {
			continue
		}
		// Gambar tanpa caption hanya diproses config yang membaca gambar.
		if !shouldHandleForAI(text, hasImage, cfg.ReadImages) {
			continue
		}
		if !matchTriggerKeywords(text, cfg.TriggerKeywords) {
			continue
		}

		// Proteksi prompt injection: pola jailbreak ditolak langsung dengan
		// pesan sopan tanpa memanggil AI (hemat biaya & deterministik).
		if cfg.InjectionGuard && service.DetectPromptInjection(text) {
			log.Warn().
				Uint("deviceID", sess.DeviceID).
				Uint("configID", cfg.ID).
				Str("to", senderJID).
				Msg("AI reply blocked: prompt injection detected")
			m.sendChatbotReply(sess, senderJID, senderPhone, text,
				service.InjectionRefusalMessage, "blocked", nil)
			m.RecordFirstResponse(sess.UserID, sess.DeviceID, senderPhone)
			return
		}

		// Jalur chatbot (Pro/Master): FAQ → StrictFAQ → LLM + konteks.
		if allowed, _ := service.ChatbotAllowed(m.db, sess.UserID); allowed {
			m.replyChatbot(sess, senderJID, senderPhone, text, &cfg, msg)
			return
		}

		// Jalur lama (paket lain): LLM langsung tanpa FAQ/konteks/gambar —
		// pesan gambar tanpa caption tidak punya teks untuk dikirim.
		if strings.TrimSpace(text) == "" {
			continue
		}
		reply, err := m.askAIForUser(sess.UserID, &cfg, text)
		if err != nil {
			log.Error().Err(err).
				Uint("deviceID", sess.DeviceID).
				Uint("configID", cfg.ID).
				Msg("AI reply failed")
			continue
		}
		if strings.TrimSpace(reply) == "" {
			continue
		}
		m.sendChatbotReply(sess, senderJID, senderPhone, text,
			service.TruncateReply(reply, service.EffectiveMaxReplyLength(&cfg)), "llm", nil)
		m.RecordFirstResponse(sess.UserID, sess.DeviceID, senderPhone)
		return // hanya config pertama yang cocok yang membalas
	}
}

// replyChatbot menjalankan pipeline chatbot untuk satu config:
// retrieval FAQ, lalu (bila perlu) fallback LLM dengan konteks percakapan.
func (m *Manager) replyChatbot(sess *SessionState, senderJID, senderPhone, text string, cfg *models.AIReplyConfig, msg *events.Message) {
	maxLen := service.EffectiveMaxReplyLength(cfg)
	// Teks untuk log audit: gambar tanpa caption dicatat "[gambar]".
	question := aiAuditQuestion(text, isImageMessage(msg))

	// 1. Retrieval FAQ.
	if cfg.UseFAQ {
		faqs := service.LoadFAQs(m.db, sess.UserID, sess.DeviceID)
		if best, _ := service.FindBestFAQ(faqs, text); best != nil {
			answer := service.TruncateReply(best.Answer, maxLen)
			faqID := best.ID
			if m.sendChatbotReply(sess, senderJID, senderPhone, question, answer, "faq", &faqID) {
				m.RecordFirstResponse(sess.UserID, sess.DeviceID, senderPhone)
			}
			return
		}
		// 2. Mode ketat: tolak topik di luar FAQ tanpa memanggil LLM.
		if cfg.StrictFAQ {
			if m.sendChatbotReply(sess, senderJID, senderPhone, question,
				service.OutOfScopeRefusalMessage, "refused", nil) {
				m.RecordFirstResponse(sess.UserID, sess.DeviceID, senderPhone)
			}
			return
		}
	}

	// 3a. Gambar (opsional): diunduh LAZY — baru di sini, setelah FAQ/StrictFAQ
	// tidak menjawab. Gagal baca → degradasi ke teks saja, atau pesan sopan
	// bila tak ada caption sebagai pegangan.
	hasCaption := strings.TrimSpace(text) != ""
	var images []service.ChatImage
	imageSkipped := false
	if cfg.ReadImages && isImageMessage(msg) {
		// Model yang SUDAH terbukti tak bisa melihat gambar (hasil "Cek Gambar"):
		// jangan unduh/kirim gambar; model menjawab teks dengan jujur (catatan sistem).
		conn := service.ConnectionForUser(m.db, sess.UserID)
		if decideImagePath(conn) == imageSkip {
			imageSkipped = true
			log.Info().Uint("deviceID", sess.DeviceID).Uint("configID", cfg.ID).
				Msg("AI reply: gambar dilewati — model tercatat tidak mendukung gambar")
		}
	}
	if cfg.ReadImages && isImageMessage(msg) && !imageSkipped {
		img, err := m.loadAIImage(sess, msg)
		if err != nil {
			log.Warn().Err(err).
				Uint("deviceID", sess.DeviceID).
				Uint("configID", cfg.ID).
				Msg("AI reply: gambar tak bisa dibaca")
			if !hasCaption {
				m.replyImageUnreadable(sess, senderJID, senderPhone, question)
				return
			}
		} else {
			images = []service.ChatImage{img}
		}
	}

	// 3b. Fallback LLM dengan konteks N pesan terakhir percakapan ini.
	history := service.RecentContext(m.db, sess.UserID, sess.DeviceID, senderPhone,
		service.EffectiveContextMessages(cfg))
	askCfg, askText := cfg, text
	if imageSkipped {
		// Catatan jujur di system prompt; teks pelanggan tak diubah, kecuali kosong
		// (tanpa caption) -> teks pengganti agar pesan user tak kosong.
		c2 := *cfg
		c2.SystemPrompt = withImageUnsupportedNote(cfg.SystemPrompt)
		askCfg = &c2
		if !hasCaption {
			askText = imageSkippedUserText
		}
	}
	reply, err := m.askAIWithHistory(sess.UserID, askCfg, history, askText, images)
	if len(images) > 0 && (err != nil || strings.TrimSpace(reply) == "") {
		// Model kemungkinan tak mendukung gambar (HTTP 400) atau menjawab kosong
		// (token habis berpikir / diblokir): coba lagi tanpa gambar bila ada
		// caption, kalau tidak balas sopan — pelanggan tak boleh dibiarkan diam.
		log.Warn().Err(err).
			Uint("deviceID", sess.DeviceID).
			Uint("configID", cfg.ID).
			Msg("AI reply dengan gambar gagal/kosong — model AI mungkin tidak mendukung gambar")
		if !hasCaption {
			m.replyImageUnreadable(sess, senderJID, senderPhone, question)
			return
		}
		// Ulangan tanpa gambar juga TANPA pencarian: satu pesan pelanggan tidak
		// boleh memicu dua kali pencarian berbiaya.
		retryCfg := *cfg
		retryCfg.WebSearch = false
		reply, err = m.askAIWithHistory(sess.UserID, &retryCfg, history, text, nil)
	}
	if err != nil {
		log.Error().Err(err).
			Uint("deviceID", sess.DeviceID).
			Uint("configID", cfg.ID).
			Msg("AI reply failed")
		return
	}
	if strings.TrimSpace(reply) == "" {
		return
	}
	if m.sendChatbotReply(sess, senderJID, senderPhone, question,
		service.TruncateReply(reply, maxLen), "llm", nil) {
		m.RecordFirstResponse(sess.UserID, sess.DeviceID, senderPhone)
	}
}

// replyImageUnreadable membalas sopan saat gambar tanpa caption tak bisa dibaca.
func (m *Manager) replyImageUnreadable(sess *SessionState, senderJID, senderPhone, question string) {
	if m.sendChatbotReply(sess, senderJID, senderPhone, question, ImageUnreadableMessage, "refused", nil) {
		m.RecordFirstResponse(sess.UserID, sess.DeviceID, senderPhone)
	}
}

// matchTriggerKeywords: keywords kosong = cocok untuk semua pesan,
// selain itu cocok bila salah satu keyword muncul di teks (case-insensitive).
func matchTriggerKeywords(text, keywords string) bool {
	if strings.TrimSpace(keywords) == "" {
		return true
	}
	return matchKeyword(text, keywords, "contains")
}

// askAIForUser mengirim prompt ke AI memakai koneksi milik user (provider +
// API key + model pilihannya sendiri). Bila user belum mengonfigurasi koneksi,
// fallback ke backend AI global (legacy AskAI via 9router/settings).
// Bila cfg.InjectionGuard aktif: system prompt diperkuat akhiran keamanan dan
// pesan user dibungkus sebagai data eksplisit.
func (m *Manager) askAIForUser(userID uint, cfg *models.AIReplyConfig, userMessage string) (string, error) {
	// Jalur paket non-Pro/Master: tanpa gambar dan TANPA pencarian web (fitur
	// berbiaya itu hanya untuk chatbot Pro/Master) — salin config, jangan ubah aslinya.
	legacy := *cfg
	legacy.WebSearch = false
	return m.askAIWithHistory(userID, &legacy, nil, userMessage, nil)
}

// composeAIPrompt menyusun system prompt + pesan user untuk LLM.
// Gambar tanpa caption diberi teks pengantar agar pesan user tidak kosong.
// Pelindung injection mengikuti cfg.InjectionGuard; ImageGuardSuffix hanya
// ditambahkan bila pesan membawa gambar.
func composeAIPrompt(systemPrompt, userText string, injectionGuard, withImage bool) (system, user string) {
	if withImage && strings.TrimSpace(userText) == "" {
		userText = imageOnlyPlaceholder
	}
	system, user = systemPrompt, userText
	if injectionGuard {
		system = service.GuardedSystemPrompt(systemPrompt)
		if withImage {
			system += "\n" + service.ImageGuardSuffix
		}
		user = service.GuardedUserMessage(userText)
	}
	return system, user
}

// assembleMessages menyusun daftar pesan ke LLM: satu system prompt (dengan baris
// waktu di akhir), riwayat yang sudah dinormalkan, lalu pesan user terakhir
// (membawa gambar bila ada). Dipisah agar urutan & baris waktu bisa dites.
func assembleMessages(system string, history []service.ChatMessage, userMsg string, images []service.ChatImage, now time.Time) []service.ChatMessage {
	msgs := []service.ChatMessage{{Role: "system", Content: withClock(system, now)}}
	msgs = append(msgs, service.NormalizeChatHistory(history)...)
	return append(msgs, service.ChatMessage{Role: "user", Content: userMsg, Images: images})
}

// askAIWithHistory sama dengan askAIForUser, ditambah riwayat percakapan
// (terurut kronologis, tanpa system prompt) sebagai konteks sebelum pesan
// terbaru, dan gambar opsional yang menempel di pesan user terakhir.
// Riwayat dinormalkan (gabung role berurutan, buang assistant di ujung)
// agar diterima semua provider.
func (m *Manager) askAIWithHistory(userID uint, cfg *models.AIReplyConfig, history []service.ChatMessage, userMessage string, images []service.ChatImage) (string, error) {
	systemPrompt, guardedMsg := composeAIPrompt(cfg.SystemPrompt, userMessage, cfg.InjectionGuard, len(images) > 0)
	// Waktu diambil SEKALI per panggilan agar balasan awal dan ulangan (setelah
	// penurunan fitur) memakai jam yang sama.
	now := time.Now()
	// Cari di Web lewat penyedia pencarian terpisah (tidak terikat model): hasil/catatan
	// ditempel ke system prompt sebagai DATA. Bila sudah mencari/gagal lewat jalur
	// ini, pencarian bawaan provider TIDAK dipakai lagi (tak mencari dua kali).
	// Teks pengganti (gambar dilewati tanpa caption) bukan pertanyaan pelanggan:
	// jangan dijadikan kueri pencarian.
	searchNote := ""
	if userMessage != imageSkippedUserText {
		searchNote, _ = webSearchNote(context.Background(), m.db, service.DefaultSearchGuard,
			service.SearcherForConnection, userID, cfg, userMessage)
	}
	serverSearch := searchNote != ""
	if serverSearch {
		systemPrompt = withSystemNote(systemPrompt, searchNote)
	}
	build := func(system string) []service.ChatMessage {
		return assembleMessages(system, history, guardedMsg, images, now)
	}

	if conn := service.ConnectionForUser(m.db, userID); conn != nil {
		if svc := service.ServiceForConnection(conn); svc != nil {
			provider := service.AIProvider(conn.Provider)
			model := service.EffectiveModel(conn, "")
			// Pencarian web hanya bila config memintanya DAN provider mendukung
			// (Z.AI/Anthropic); selain itu diabaikan, jawaban tetap dari model.
			webSearch := cfg.WebSearch && !serverSearch && service.WebSearchSupported(provider)
			if cfg.WebSearch && !serverSearch && !webSearch {
				log.Warn().Uint("userID", userID).Str("provider", conn.Provider).
					Msg("Cari di Web aktif tetapi provider tidak mendukung — diabaikan")
			}
			sys := systemPrompt
			if webSearch {
				sys = service.WithWebSearchGuard(systemPrompt)
			}
			req := service.ChatRequest{
				Provider:        provider,
				Model:           model,
				Messages:        build(sys),
				MaxTokens:       service.AIReplyMaxTokens,
				Temperature:     0.7,
				ReasoningEffort: service.ReasoningEffortFor(provider, model),
				WebSearch:       webSearch,
			}
			if webSearch {
				// Bila provider menolak tools dan kita mengulang tanpa pencarian,
				// pakai system prompt TANPA kalimat "alat pencarian tersedia".
				req.DowngradeMessages = build(systemPrompt)
			}
			resp, err := svc.CompleteTolerant(req)
			if err != nil {
				return "", err
			}
			if resp.Downgraded {
				log.Warn().Uint("userID", userID).Str("provider", conn.Provider).Str("model", model).
					Msg("Provider menolak fitur tambahan (pencarian web/reasoning_effort) — dijawab tanpa fitur itu")
			} else if webSearch {
				log.Info().Uint("userID", userID).Str("provider", conn.Provider).Str("model", model).
					Bool("mencari", resp.Searched).Strs("sumber", resp.Sources).
					Msg("AI Reply dengan Cari di Web")
			}
			// Jawaban kosong jangan hilang diam-diam: catat sebabnya (mis. model
			// reasoning menghabiskan token untuk berpikir).
			if strings.TrimSpace(resp.Content) == "" {
				log.Warn().
					Uint("userID", userID).
					Str("provider", conn.Provider).
					Str("model", model).
					Int("tokens", resp.Tokens).
					Str("sebab", resp.EmptyHint()).
					Msg("AI membalas kosong")
			}
			return resp.Content, nil
		}
		log.Warn().Uint("userID", userID).Msg("AI connection key undecryptable, falling back to global AI backend")
	}
	// Fallback global (legacy AskAI) tidak mendukung riwayat multi-turn:
	// kirim hanya system prompt + pesan terbaru.
	return AskAI(m.db, withClock(systemPrompt, now), guardedMsg, images...)
}
