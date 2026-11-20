/* Pemformat teks ala WhatsApp: *tebal*, _miring_, ~coret~, ```monospace```, dan tautan http(s).
 * Hanya menghasilkan elemen React (tanpa dangerouslySetInnerHTML) sehingga aman dari XSS.
 * Copyright © 2026 Duta Corpora Indonesia — All rights reserved */
import type { ReactNode } from "react";

const TOKEN = /```[\s\S]+?```|\*[^*\n]+\*|_[^_\n]+_|~[^~\n]+~|https?:\/\/[^\s<]+/g;
const URL_TAIL = /[.,;:!?)\]]+$/;

export function renderWaText(text: string): ReactNode[] {
  const out: ReactNode[] = [];
  let last = 0;
  let key = 0;
  for (const m of text.matchAll(TOKEN)) {
    const start = m.index ?? 0;
    if (start > last) out.push(text.slice(last, start));
    let tok = m[0];
    last = start + tok.length;
    const k = key++;

    if (tok.startsWith("http")) {
      const tail = tok.match(URL_TAIL)?.[0] ?? "";
      tok = tok.slice(0, tok.length - tail.length);
      out.push(
        <a
          key={k}
          href={tok}
          target="_blank"
          rel="noopener noreferrer"
          className="text-[#027eb5] dark:text-[#53bdeb] underline break-all"
        >
          {tok}
        </a>
      );
      if (tail) out.push(tail);
      continue;
    }
    if (tok.startsWith("```")) {
      out.push(
        <code key={k} className="font-mono text-[12.5px] whitespace-pre-wrap">
          {tok.slice(3, -3)}
        </code>
      );
      continue;
    }
    const inner = tok.slice(1, -1);
    if (tok[0] === "*") out.push(<strong key={k}>{inner}</strong>);
    else if (tok[0] === "_") out.push(<em key={k}>{inner}</em>);
    else out.push(<s key={k}>{inner}</s>);
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}
