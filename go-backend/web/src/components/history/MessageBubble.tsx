/* Gelembung pesan keluar ala WhatsApp untuk pratinjau Riwayat Pesan.
 * Copyright © 2026 Duta Corpora Indonesia — All rights reserved */
import { AlertCircle, Clock, FileText, MapPin } from "lucide-react";
import { renderWaText } from "@/lib/wa-text";

export interface PreviewMessage {
  id: number;
  deviceId: number;
  to: string;
  type: string;
  content: string;
  caption?: string;
  mediaUrl?: string;
  errorMsg?: string;
  status: string;
  via: string;
  messageId: string;
  createdAt: string;
}

// Hanya URL publik / path situs yang boleh dimuat; file:// = path server, jangan disentuh.
const isSafeUrl = (u?: string) => !!u && /^(https?:\/\/|\/(?!\/))/i.test(u);

function Ticks({ status }: { status: string }) {
  const s = status.toLowerCase();
  if (s === "failed") return <AlertCircle className="w-3.5 h-3.5 text-red-500" aria-hidden />;
  if (s === "pending") return <Clock className="w-3 h-3 text-[#8696a0]" aria-hidden />;
  const double = s === "delivered" || s === "read";
  return (
    <svg
      viewBox="0 0 18 14"
      className={`w-4 h-3.5 ${s === "read" ? "text-[#53bdeb]" : "text-[#8696a0]"}`}
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden
    >
      <path d="M1 7.5 4 10.5 11 3" />
      {double && <path d="M7 7.5 10 10.5 17 3" />}
    </svg>
  );
}

export function MessageBubble({ m, time, typeLabel }: { m: PreviewMessage; time: string; typeLabel: string }) {
  const type = (m.type || "text").toLowerCase();
  const url = isSafeUrl(m.mediaUrl) ? m.mediaUrl : undefined;
  const raw = (m.caption || m.content || "").trim();
  const text = raw === m.mediaUrl ? "" : raw;
  const fileName = m.mediaUrl ? m.mediaUrl.split("/").pop() || typeLabel : typeLabel;

  return (
    <div className="relative max-w-[85%] ml-auto">
      <svg viewBox="0 0 8 13" className="absolute -right-[7px] top-0 w-2 h-[13px] fill-[#d9fdd3] dark:fill-[#005c4b]" aria-hidden>
        <path d="M0 0 L8 0 L0 13 Z" />
      </svg>
      <div className="rounded-[7.5px] rounded-tr-none bg-[#d9fdd3] dark:bg-[#005c4b] px-2 pt-1.5 pb-1 text-[14px] leading-snug text-[#111b21] dark:text-[#e9edef] shadow-[0_1px_1px_rgba(0,0,0,0.13)]">
        {url && (type === "image" || type === "sticker") && (
          <img src={url} alt="" loading="lazy" className="rounded-md mb-1 max-h-72 w-full object-cover" />
        )}
        {url && type === "video" && <video src={url} controls className="rounded-md mb-1 max-h-72 w-full bg-black" />}
        {url && (type === "audio" || type === "voicenote" || type === "voice") && (
          <audio src={url} controls className="my-1 w-60 max-w-full" />
        )}
        {url && type === "document" && (
          <a href={url} target="_blank" rel="noopener noreferrer" className="mb-1 flex items-center gap-2.5 rounded-md bg-black/5 dark:bg-white/10 p-2">
            <FileText className="w-7 h-7 shrink-0 text-[#667781] dark:text-[#aebac1]" aria-hidden />
            <span className="min-w-0 truncate text-[13px]">{fileName}</span>
          </a>
        )}
        {!url && type !== "text" && type !== "location" && (
          <p className="mb-1 text-[13px] italic opacity-80">{typeLabel}</p>
        )}
        {type === "location" && (
          <p className="mb-1 flex items-center gap-1.5 text-[13px]">
            <MapPin className="w-4 h-4 shrink-0 text-red-500" aria-hidden /> {m.content}
          </p>
        )}
        {text && type !== "location" && (
          <p className="whitespace-pre-wrap break-words">{renderWaText(text)}</p>
        )}
        <span className="float-right -mt-0.5 ml-3 mb-0.5 flex items-center gap-1 text-[10.5px] leading-none text-[#667781] dark:text-[#e9edef]/70">
          {time}
          <Ticks status={m.status || ""} />
        </span>
        <span className="clear-both block" />
      </div>
    </div>
  );
}
