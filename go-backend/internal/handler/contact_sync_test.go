// Test pemetaan peserta grup WA → nomor HP (contact_sync.go).
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
package handler

import (
	"reflect"
	"testing"

	"go.mau.fi/whatsmeow/types"
)

// Regresi produksi: semua grup tersinkron dengan 0 anggota karena WhatsApp
// memakai alamat LID di grup, sedangkan kode lama hanya menerima @s.whatsapp.net.
func TestGroupParticipantPhones(t *testing.T) {
	pn := func(u string) types.JID { return types.NewJID(u, types.DefaultUserServer) }
	lid := func(u string) types.JID { return types.NewJID(u, types.HiddenUserServer) }
	// map LID → nomor asli (cache session store)
	cache := map[string]string{"900000000000003": "6285555000003"}
	resolve := func(u string) string { return cache[u] }

	cases := []struct {
		name           string
		parts          []types.GroupParticipant
		resolve        func(string) string
		wantPhones     []string
		wantUnresolved int
	}{
		{
			name:       "peserta format nomor lama",
			parts:      []types.GroupParticipant{{JID: pn("6281111000001")}},
			wantPhones: []string{"6281111000001"},
		},
		{
			name:       "LID dengan PhoneNumber terisi (kasus produksi)",
			parts:      []types.GroupParticipant{{JID: lid("900000000000001"), LID: lid("900000000000001"), PhoneNumber: pn("6282222000002")}},
			wantPhones: []string{"6282222000002"},
		},
		{
			name:       "LID tanpa PhoneNumber, dipetakan lewat cache",
			parts:      []types.GroupParticipant{{JID: lid("900000000000003"), LID: lid("900000000000003")}},
			resolve:    resolve,
			wantPhones: []string{"6285555000003"},
		},
		{
			name:           "LID tak bisa dipetakan dihitung, bukan hilang diam-diam",
			parts:          []types.GroupParticipant{{JID: lid("900000000000009"), LID: lid("900000000000009")}},
			resolve:        resolve,
			wantUnresolved: 1,
		},
		{
			name:           "tanpa resolver, LID tanpa nomor dihitung tak terselesaikan",
			parts:          []types.GroupParticipant{{JID: lid("900000000000008")}},
			wantUnresolved: 1,
		},
		{
			name: "campuran + duplikat dibuang, urutan asli terjaga",
			parts: []types.GroupParticipant{
				{JID: pn("6281111000001")},
				{JID: lid("900000000000001"), PhoneNumber: pn("6281111000001")}, // orang sama via LID
				{JID: lid("900000000000002"), PhoneNumber: pn("6282222000002")},
				{JID: lid("900000000000009")},
			},
			resolve:        resolve,
			wantPhones:     []string{"6281111000001", "6282222000002"},
			wantUnresolved: 1,
		},
		{
			name:       "entri bukan nomor (server lain / bukan digit) dilewati",
			parts:      []types.GroupParticipant{{JID: types.NewJID("abc", types.DefaultUserServer)}, {JID: types.NewJID("123", types.GroupServer)}, {JID: pn("6283333000003")}},
			wantPhones: []string{"6283333000003"},
		},
		{name: "grup kosong", parts: nil},
	}
	for _, tc := range cases {
		gotPhones, gotUnres := groupParticipantPhones(tc.parts, tc.resolve)
		if len(gotPhones) == 0 && len(tc.wantPhones) == 0 {
			gotPhones = nil
		}
		if !reflect.DeepEqual(gotPhones, tc.wantPhones) {
			t.Errorf("%s: nomor = %v, mau %v", tc.name, gotPhones, tc.wantPhones)
		}
		if gotUnres != tc.wantUnresolved {
			t.Errorf("%s: tak terselesaikan = %d, mau %d", tc.name, gotUnres, tc.wantUnresolved)
		}
	}
}
