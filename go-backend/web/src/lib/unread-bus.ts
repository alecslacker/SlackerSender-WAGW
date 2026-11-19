// Pemancar kecil agar badge menu Live Chat (Sidebar) dan halaman Live Chat
// berbagi SATU sumber angka: server (GET /chat/unread-count). Halaman Live
// Chat cukup memanggil notifyUnreadChanged() setiap kali ada pesan masuk /
// percakapan dibaca; Sidebar mengambil ulang angkanya dari server.
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia

const EVENT = "wag:unread-changed";

/** Beri tahu semua pendengar bahwa angka belum-dibaca mungkin berubah. */
export function notifyUnreadChanged(): void {
  if (typeof window !== "undefined") window.dispatchEvent(new Event(EVENT));
}

/** Dengarkan perubahan; mengembalikan fungsi untuk berhenti mendengarkan. */
export function onUnreadChanged(handler: () => void): () => void {
  if (typeof window === "undefined") return () => {};
  window.addEventListener(EVENT, handler);
  return () => window.removeEventListener(EVENT, handler);
}
