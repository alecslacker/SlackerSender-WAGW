import React from "react";
import ReactDOM from "react-dom/client";
import { Router } from "wouter";
import { LanguageProvider } from "@/lib/i18n";
import { ThemeProvider } from "@/hooks/use-theme";
import Landing from "@/pages/Landing";
import "./index.css";

// Kalau sudah login, langsung ke dashboard (landing hanya untuk publik).
try {
  if (localStorage.getItem("token")) {
    window.location.replace("/dashboard");
  }
} catch {
  /* abaikan */
}

const rootEl = document.getElementById("root");
if (rootEl && !localStorage.getItem("token")) {
  ReactDOM.createRoot(rootEl).render(
    <React.StrictMode>
      {/* ThemeProvider memasang class "dark" + data-mode di <html> —
          tanpa ini token tema terang yang aktif di halaman "/" produksi. */}
      <ThemeProvider>
        <LanguageProvider>
          <Router>
            <Landing />
          </Router>
        </LanguageProvider>
      </ThemeProvider>
    </React.StrictMode>
  );
}
