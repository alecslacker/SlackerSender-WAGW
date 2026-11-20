/* Popup pratinjau pesan ala WhatsApp untuk Riwayat Pesan.
 * Copyright © 2026 Duta Corpora Indonesia — All rights reserved */
import { useEffect, useRef } from "react";
import { createPortal } from "react-dom";
import { ChevronLeft, ChevronRight, User, X } from "lucide-react";
import { useLang } from "@/lib/i18n";
import { WA_DOODLE_LIGHT, WA_DOODLE_DARK } from "@/lib/wa-doodle";
import { MessageBubble, type PreviewMessage } from "./MessageBubble";

interface Props {
  message: PreviewMessage;
  deviceLabel: string;
  hasPrev: boolean;
  hasNext: boolean;
  onPrev: () => void;
  onNext: () => void;
  onClose: () => void;
}

const TYPE_KEYS: Record<string, string> = {
  image: "typeImage",
  video: "typeVideo",
  audio: "typeAudio",
  voicenote: "typeAudio",
  voice: "typeAudio",
  document: "typeDocument",
  sticker: "typeSticker",
  location: "typeLocation",
  poll: "typePoll",
  interactive: "typeInteractive",
  buttons: "typeInteractive",
};

// Sama dengan label badge di tabel Riwayat ("sent" tampil sebagai Terkirim).
const STATUS_KEYS: Record<string, string> = {
  read: "statusRead",
  delivered: "statusDelivered",
  sent: "statusDelivered",
  failed: "statusFailed",
  revoked: "statusRevoked",
  pending: "statusPending",
};

export function MessagePreviewDialog({ message: m, deviceLabel, hasPrev, hasNext, onPrev, onNext, onClose }: Props) {
  const { t, lang } = useLang();
  const locale = lang === "id" ? "id-ID" : "en-US";
  const closeRef = useRef<HTMLButtonElement>(null);
  const d = new Date(m.createdAt);
  const type = (m.type || "text").toLowerCase();
  const typeLabel = TYPE_KEYS[type] ? t(`history.${TYPE_KEYS[type]}`) : t("history.typeOther");

  useEffect(() => {
    const prevFocus = document.activeElement as HTMLElement | null;
    closeRef.current?.focus();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
      else if (e.key === "ArrowLeft" && hasPrev) onPrev();
      else if (e.key === "ArrowRight" && hasNext) onNext();
    };
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("keydown", onKey);
      prevFocus?.focus?.();
    };
  }, [hasPrev, hasNext, onPrev, onNext, onClose]);

  const rows: [string, string][] = [
    [t("history.headerStatus"), STATUS_KEYS[(m.status || "").toLowerCase()] ? t(`history.${STATUS_KEYS[(m.status || "").toLowerCase()]}`) : m.status || "-"],
    [t("history.headerVia"), m.via || "-"],
    [t("history.headerSender"), deviceLabel],
    [t("history.detailId"), m.messageId || `#${m.id}`],
  ];

  const navBtn = "w-9 h-9 rounded-full inline-flex items-center justify-center text-white/90 hover:bg-white/15 disabled:opacity-30 disabled:hover:bg-transparent focus:outline-none focus-visible:ring-2 focus-visible:ring-white";

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-3 sm:p-4"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-label={t("history.previewTitle")}
    >
      <div
        className="flex w-full max-w-md max-h-[90dvh] flex-col overflow-hidden rounded-xl bg-[#efeae2] dark:bg-[#0b141a] shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center gap-2 bg-[#008069] dark:bg-[#1f2c34] px-3 py-2.5 text-white">
          <span className="w-10 h-10 shrink-0 rounded-full bg-[#dfe5e7] text-[#8696a0] inline-flex items-center justify-center overflow-hidden" aria-hidden>
            <User className="w-7 h-7 translate-y-1 fill-current" />
          </span>
          <div className="min-w-0 flex-1">
            <p className="truncate text-[15px] font-medium leading-tight">{m.to}</p>
            <p className="truncate text-[12px] text-white/75">
              {t("history.previewVia").replace("{device}", deviceLabel)}
            </p>
          </div>
          <button type="button" className={navBtn} disabled={!hasPrev} onClick={onPrev} aria-label={t("history.previewPrev")}>
            <ChevronLeft className="w-5 h-5" />
          </button>
          <button type="button" className={navBtn} disabled={!hasNext} onClick={onNext} aria-label={t("history.previewNext")}>
            <ChevronRight className="w-5 h-5" />
          </button>
          <button ref={closeRef} type="button" className={navBtn} onClick={onClose} aria-label={t("common.close")}>
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="relative min-h-[180px] flex-1 overflow-y-auto">
          <div aria-hidden className="pointer-events-none absolute inset-0 dark:hidden" style={{ backgroundImage: WA_DOODLE_LIGHT }} />
          <div aria-hidden className="pointer-events-none absolute inset-0 hidden dark:block" style={{ backgroundImage: WA_DOODLE_DARK }} />
          <div className="relative space-y-3 px-4 py-4 pr-5">
            <div className="flex justify-center">
              <span className="rounded-lg bg-white/90 dark:bg-[#182229] px-3 py-1 text-[12px] text-[#54656f] dark:text-[#8696a0] shadow-sm">
                {d.toLocaleDateString(locale, { weekday: "long", day: "numeric", month: "long", year: "numeric" })}
              </span>
            </div>
            <MessageBubble m={m} time={d.toLocaleTimeString(locale, { hour: "2-digit", minute: "2-digit" })} typeLabel={typeLabel} />
            {m.status?.toLowerCase() === "failed" && m.errorMsg && (
              <p className="rounded-lg bg-red-50 dark:bg-red-950/40 px-3 py-2 text-[12.5px] text-red-700 dark:text-red-300 break-words">
                {t("history.detailError")}: {m.errorMsg}
              </p>
            )}
          </div>
        </div>

        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 bg-white dark:bg-[#111b21] px-4 py-3 text-[12.5px]">
          {rows.map(([k, v]) => (
            <div key={k} className="contents">
              <dt className="text-[#667781] dark:text-[#8696a0]">{k}</dt>
              <dd className="min-w-0 break-all text-right text-[#111b21] dark:text-[#e9edef]">{v}</dd>
            </div>
          ))}
        </dl>
      </div>
    </div>,
    document.body
  );
}
