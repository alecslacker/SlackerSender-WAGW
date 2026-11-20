package models

import (
	"testing"
	"time"
)

func TestResetVisionIfChanged(t *testing.T) {
	now := time.Now()
	base := func() AIConnection {
		return AIConnection{Provider: "openai", Model: "m1", BaseURL: "https://a.example",
			VisionStatus: CapabilityYes, VisionModel: "m1", VisionCheckedAt: &now}
	}
	kasus := []struct {
		nama                 string
		provider, model, url string
		keyBaru              bool
		wantReset            bool
	}{
		{"sama persis", "openai", "m1", "https://a.example", false, false},
		{"kunci diganti", "openai", "m1", "https://a.example", true, true},
		{"url diganti", "openai", "m1", "https://b.example", false, true},
		{"model diganti", "openai", "m2", "https://a.example", false, true},
		{"provider diganti", "custom", "m1", "https://a.example", false, true},
	}
	for _, k := range kasus {
		c := base()
		c.ResetVisionIfChanged(k.provider, k.model, k.url, k.keyBaru)
		reset := c.VisionStatus == "" && c.VisionModel == "" && c.VisionCheckedAt == nil
		if reset != k.wantReset {
			t.Errorf("%s: reset=%v, mau %v", k.nama, reset, k.wantReset)
		}
	}
}

func TestResetSearchIfChanged(t *testing.T) {
	now := time.Now()
	base := func() AISearchConnection {
		return AISearchConnection{Provider: "brave", BaseURL: "",
			SearchStatus: CapabilityYes, CheckedProvider: "brave", CheckedAt: &now}
	}
	kasus := []struct {
		nama      string
		provider  string
		url       string
		keyBaru   bool
		wantReset bool
	}{
		{"sama persis", "brave", "", false, false},
		{"kunci diganti", "brave", "", true, true},
		{"penyedia diganti", "zai", "", false, true},
		{"url diganti", "brave", "https://s.example", false, true},
	}
	for _, k := range kasus {
		c := base()
		c.ResetSearchIfChanged(k.provider, k.url, k.keyBaru)
		reset := c.SearchStatus == "" && c.CheckedProvider == "" && c.CheckedAt == nil
		if reset != k.wantReset {
			t.Errorf("%s: reset=%v, mau %v", k.nama, reset, k.wantReset)
		}
	}
}
