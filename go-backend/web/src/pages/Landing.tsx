import { useEffect, useRef, useState } from "react";
import {
  Zap,
  ShieldCheck,
  Headset,
  MousePointerClick,
  Send,
  Users,
  CalendarClock,
  Repeat,
  Variable,
  Paperclip,
  FileSpreadsheet,
  MessagesSquare,
  Bot,
  Webhook,
  Smartphone,
  ContactRound,
  History,
  LayoutTemplate,
  Radio,
  CalendarDays,
  ChartLine,
  Code2,
  KeyRound,
  BookOpen,
  Check,
  ChevronDown,
  Star,
  Menu,
  X,
  BellRing,
  Timer,
  Play,
  FlaskConical,
  ChevronRight,
  MapPin,
  CircleHelp,
} from "lucide-react";
import { useLang } from "@/lib/i18n";

/* Link = anchor biasa (muat halaman penuh). Landing disajikan dari bundle
 * terpisah (landing-entry.tsx) yang TIDAK punya <Route>; Link wouter hanya
 * pushState sehingga URL berubah ke /login tapi layar tidak berganti. */
function Link({
  href,
  children,
  ...rest
}: { href: string } & React.AnchorHTMLAttributes<HTMLAnchorElement>) {
  return (
    <a href={href} {...rest}>
      {children}
    </a>
  );
}

/* ── Util: reveal on scroll ─────────────────────────── */
/* Reveal: animasi scroll berbasis CSS class (bukan state) agar ramah prerender —
 * tanpa JS konten langsung terlihat; dengan JS, class reveal-js menyembunyikan
 * dulu lalu revealed memunculkan saat masuk viewport. Tidak ada mismatch
 * hidrasi karena struktur DOM awal identik. */
function Reveal({ children, delay = 0, className = "" }: { children: React.ReactNode; delay?: number; className?: string }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.classList.add("reveal-js");
    const io = new IntersectionObserver(
      ([e]) => {
        if (e.isIntersecting) {
          el.classList.add("revealed");
          io.disconnect();
        }
      },
      { threshold: 0.12 }
    );
    io.observe(el);
    return () => io.disconnect();
  }, []);
  return (
    <div
      ref={ref}
      style={delay ? { transitionDelay: `${delay}ms` } : undefined}
      className={`reveal ${className}`}
    >
      {children}
    </div>
  );
}

function SectionHead({ kicker, title, desc }: { kicker: string; title: string; desc?: string }) {
  return (
    <Reveal className="text-center max-w-2xl mx-auto mb-12">
      <p className="text-xs font-bold uppercase tracking-[0.2em] text-[#4ade80] mb-3">{kicker}</p>
      <h2 className="text-3xl md:text-4xl font-extrabold tracking-tight text-foreground">{title}</h2>
      {desc && <p className="text-muted-foreground mt-3">{desc}</p>}
    </Reveal>
  );
}

/* ── Navbar ─────────────────────────────────────────── */
function Navbar() {
  const { t } = useLang();
  const navLinks = [
    { label: t("landing.navFeatures"), href: "#fitur" },
    { label: t("landing.navPricing"), href: "#harga" },
    { label: t("landing.navFaq"), href: "#faq" },
  ];
  const [open, setOpen] = useState(false);
  const [scrolled, setScrolled] = useState(false);
  useEffect(() => {
    const fn = () => setScrolled(window.scrollY > 12);
    window.addEventListener("scroll", fn, { passive: true });
    return () => window.removeEventListener("scroll", fn);
  }, []);
  return (
    <header className={`fixed top-0 inset-x-0 z-50 transition-all ${scrolled ? "bg-background/90 backdrop-blur-md shadow-sm" : "bg-transparent"}`}>
      <div className="max-w-6xl mx-auto px-4 h-16 flex items-center justify-between">
        <a href="#top" className="flex items-center gap-2.5">
          <img src="/slackercode-logo.png" alt="Logo SlackerSender" className="h-9 w-9" />
          <span className="text-xl font-extrabold tracking-tight text-[#4ade80]">SlackerSender</span>
        </a>
        <nav className="hidden md:flex items-center gap-8">
          {navLinks.map((l) => (
            <a key={l.href} href={l.href} className="text-sm font-medium text-muted-foreground hover:text-[#4ade80] transition-colors">
              {l.label}
            </a>
          ))}
          <Link href="/api-docs" className="text-sm font-medium text-muted-foreground hover:text-[#4ade80] transition-colors">
            {t("landing.docs")}
          </Link>
        </nav>
        <div className="hidden md:flex items-center gap-2">
          <Link href="/login" className="text-sm font-semibold text-foreground hover:text-[#4ade80] px-3 py-2">
            {t("landing.login")}
          </Link>
          <Link href="/register" className="text-sm font-semibold bg-[#15803d] hover:bg-[#0b3d1f] text-white px-4 py-2 rounded-lg transition-colors">
            {t("landing.tryFree")}
          </Link>
        </div>
        <button className="md:hidden p-2 text-foreground" onClick={() => setOpen((v) => !v)} aria-label={t("landing.menu")}>
          {open ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
        </button>
      </div>
      {open && (
        <div className="md:hidden bg-background border-t border-border px-4 py-4 space-y-1 shadow-lg">
          {navLinks.map((l) => (
            <a key={l.href} href={l.href} onClick={() => setOpen(false)} className="block px-2 py-2 text-sm font-medium text-foreground hover:text-[#4ade80]">
              {l.label}
            </a>
          ))}
          <Link href="/api-docs" onClick={() => setOpen(false)} className="block px-2 py-2 text-sm font-medium text-foreground">
            {t("landing.docs")}
          </Link>
          <div className="flex gap-2 pt-2">
            <Link href="/login" onClick={() => setOpen(false)} className="flex-1 text-center text-sm font-semibold border border-border rounded-lg px-3 py-2">
              {t("landing.login")}
            </Link>
            <Link href="/register" onClick={() => setOpen(false)} className="flex-1 text-center text-sm font-semibold bg-[#15803d] text-white rounded-lg px-3 py-2">
              {t("landing.tryFree")}
            </Link>
          </div>
        </div>
      )}
    </header>
  );
}

/* ── Hero: animasi alur gateway (render Remotion, sumber di /motion) ────────
 * Developer kirim lewat API → gateway SlackerSender → pesan WhatsApp tiba di
 * ponsel → webhook balik. Video loop 10 dtk; poster = frame akhir cerita.
 * prefers-reduced-motion → hanya poster statis (tanpa memutar video). */
function HeroArt() {
  const { t } = useLang();
  const [reduceMotion, setReduceMotion] = useState(false);

  useEffect(() => {
    const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
    setReduceMotion(mq.matches);
    const onChange = (e: MediaQueryListEvent) => setReduceMotion(e.matches);
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  return (
    <div className="relative mx-auto w-full max-w-5xl">
      {/* Glow lembut di belakang bingkai */}
      <div aria-hidden className="absolute -inset-6 rounded-[2rem] bg-[#22c55e]/10 blur-3xl" />
      <div className="relative overflow-hidden rounded-2xl border border-border bg-[#050805] shadow-[0_40px_90px_-30px_rgba(0,0,0,0.9)] aspect-[16/9]">
        {reduceMotion ? (
          <img
            src="/hero/hero-poster.webp"
            alt={t("landing.heroArtAlt")}
            className="absolute inset-0 h-full w-full object-cover"
            loading="eager"
            fetchPriority="high"
          />
        ) : (
          <video
            className="absolute inset-0 h-full w-full object-cover"
            poster="/hero/hero-poster.webp"
            autoPlay
            loop
            muted
            playsInline
            preload="auto"
            aria-label={t("landing.heroArtAlt")}
          >
            <source src="/hero/hero-loop.webm" type="video/webm" />
            <source src="/hero/hero-loop.mp4" type="video/mp4" />
          </video>
        )}
      </div>
    </div>
  );
}

/* ── Latar terminal hero: matrix rain + grid + scanline ─────────────────────── */
function TerminalBackdrop() {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    // Hormati preferensi user yang menonaktifkan animasi.
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    const parent = canvas.parentElement as HTMLElement;
    const FONT = 14;
    const CHARS = "01{}<>[]$#*+=/;ABCDEF";
    let raf = 0;
    let cols = 0;
    let drops: number[] = [];

    const resize = () => {
      canvas.width = parent.clientWidth;
      canvas.height = parent.clientHeight;
      cols = Math.ceil(canvas.width / FONT);
      drops = Array.from({ length: cols }, () =>
        Math.floor(Math.random() * (canvas.height / FONT))
      );
      ctx.font = `${FONT}px 'JetBrains Mono Variable', 'JetBrains Mono', monospace`;
    };
    resize();
    const ro = new ResizeObserver(resize);
    ro.observe(parent);

    const tick = () => {
      // Pudarkan jejak lama tanpa menutupi latar (canvas tetap transparan).
      ctx.globalCompositeOperation = "destination-out";
      ctx.fillStyle = "rgba(0,0,0,0.08)";
      ctx.fillRect(0, 0, canvas.width, canvas.height);
      ctx.globalCompositeOperation = "source-over";
      for (let i = 0; i < cols; i++) {
        const ch = CHARS[Math.floor(Math.random() * CHARS.length)];
        ctx.fillStyle = Math.random() < 0.08 ? "#b7fcc6" : "#22c55e";
        ctx.fillText(ch, i * FONT, drops[i] * FONT);
        drops[i]++;
        if (drops[i] * FONT > canvas.height && Math.random() > 0.985) drops[i] = 0;
      }
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => {
      cancelAnimationFrame(raf);
      ro.disconnect();
    };
  }, []);

  return (
    <div aria-hidden className="absolute inset-0 overflow-hidden pointer-events-none">
      <div className="terminal-grid absolute inset-0" />
      <canvas ref={canvasRef} className="absolute inset-0 h-full w-full opacity-[0.16]" />
      <div className="terminal-scanlines absolute inset-0" />
      <div className="terminal-sweep absolute inset-x-0 top-0 h-40" />
    </div>
  );
}

function Hero() {
  const { t } = useLang();
  return (
    <section id="top" className="relative overflow-hidden bg-background pt-28 pb-16 md:pt-36 md:pb-20">
      <div aria-hidden className="absolute -top-56 left-1/2 -translate-x-1/2 w-[200vw] max-w-[1100px] aspect-square bg-muted rounded-full" />
      <TerminalBackdrop />
      <div className="relative max-w-6xl mx-auto px-4 text-center">
        <Reveal>
          <h1
            className="glitch-text font-display text-6xl sm:text-7xl md:text-8xl lg:text-9xl font-normal tracking-wide leading-[0.9] text-[#4ade80]"
            data-text="SLACKERSENDER"
          >
            SLACKERSENDER
          </h1>
          <p className="text-2xl md:text-3xl font-extrabold tracking-wide text-emerald-600 mt-3">
            WHATSAPP API
          </p>
          <p className="text-muted-foreground text-lg mt-4">
            {t("landing.heroSubtitle")}
          </p>
          <div className="mt-8 flex justify-center">
            <Link href="/register" className="inline-flex items-center gap-2 bg-[#15803d] hover:bg-[#0b3d1f] text-white font-semibold px-8 py-4 rounded-2xl transition-colors">
              <Send className="w-5 h-5" /> {t("landing.tryFree")}
            </Link>
          </div>
        </Reveal>
        <Reveal delay={150} className="mt-12">
          <HeroArt />
        </Reveal>
      </div>
    </section>
  );
}

/* ── Kelebihan ──────────────────────────────────────── */
function Kelebihan() {
  const { t } = useLang();
  const items = [
    { icon: MousePointerClick, t: t("landing.advantage1Title"), d: t("landing.advantage1Desc") },
    { icon: Zap, t: t("landing.advantage2Title"), d: t("landing.advantage2Desc") },
    { icon: ShieldCheck, t: t("landing.advantage3Title"), d: t("landing.advantage3Desc") },
    { icon: Headset, t: t("landing.advantage4Title"), d: t("landing.advantage4Desc") },
  ];
  return (
    <section className="py-20 bg-background">
      <div className="max-w-6xl mx-auto px-4">
        <SectionHead kicker={t("landing.advantageKicker")} title={t("landing.advantageTitle")} desc={t("landing.advantageDesc")} />
        <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-5">
          {items.map((it, i) => (
            <Reveal key={it.t} delay={i * 80}>
              <div className="h-full bg-card hover:bg-[#0d1c10]/60 border border-border hover:border-[#86efac] rounded-2xl p-6 transition-colors">
                <span className="w-12 h-12 rounded-xl bg-[#15803d]/10 flex items-center justify-center mb-4">
                  <it.icon className="w-6 h-6 text-[#4ade80]" />
                </span>
                <h3 className="font-bold text-foreground mb-2">{it.t}</h3>
                <p className="text-sm text-muted-foreground">{it.d}</p>
              </div>
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}

/* ── Fitur (tabs ala Fonnte) ────────────────────────── */
type FiturItem = { icon: any; t: string; d: string };
function Fitur() {
  const { t } = useLang();
  const [tab, setTab] = useState("kirim");
  const fiturTabs: { id: string; label: string; items: FiturItem[] }[] = [
    {
      id: "kirim",
      label: t("landing.tabSend"),
      items: [
        { icon: Send, t: "Single", d: t("landing.sendSingleDesc") },
        { icon: Users, t: "Broadcast", d: t("landing.sendBroadcastDesc") },
        { icon: CalendarClock, t: t("landing.sendScheduledTitle"), d: t("landing.sendScheduledDesc") },
        { icon: Repeat, t: t("landing.sendRecurringTitle"), d: t("landing.sendRecurringDesc") },
        { icon: Variable, t: "Variable", d: t("landing.sendVariableDesc") },
        { icon: Paperclip, t: "Attachment", d: t("landing.sendAttachmentDesc") },
        { icon: FileSpreadsheet, t: "Import CSV", d: t("landing.sendCsvDesc") },
        { icon: Timer, t: "Follow Up", d: t("landing.sendFollowUpDesc") },
        { icon: Code2, t: "Simple API", d: t("landing.sendSimpleApiDesc") },
      ],
    },
    {
      id: "balas",
      label: t("landing.tabAutoReply"),
      items: [
        { icon: MessagesSquare, t: t("landing.replyDefaultTitle"), d: t("landing.replyDefaultDesc") },
        { icon: Bot, t: "Keyword Based", d: t("landing.replyKeywordDesc") },
        { icon: Zap, t: t("landing.replyTemplateTitle"), d: t("landing.replyTemplateDesc") },
        { icon: Webhook, t: "Webhook", d: t("landing.replyWebhookDesc") },
        { icon: BellRing, t: t("landing.replyNotifTitle"), d: t("landing.replyNotifDesc") },
        { icon: History, t: t("landing.replyHistoryTitle"), d: t("landing.replyHistoryDesc") },
      ],
    },
    {
      id: "dashboard",
      label: "Dashboard",
      items: [
        { icon: Smartphone, t: "Multi Device", d: t("landing.dashMultiDeviceDesc") },
        { icon: ContactRound, t: t("landing.dashContactsTitle"), d: t("landing.dashContactsDesc") },
        { icon: Users, t: t("landing.dashGroupingTitle"), d: t("landing.dashGroupingDesc") },
        { icon: History, t: t("landing.dashHistoryTitle"), d: t("landing.dashHistoryDesc") },
        { icon: LayoutTemplate, t: t("landing.dashTemplateTitle"), d: t("landing.dashTemplateDesc") },
        { icon: Radio, t: t("landing.dashRealtimeTitle"), d: t("landing.dashRealtimeDesc") },
        { icon: CalendarDays, t: t("landing.dashCalendarTitle"), d: t("landing.dashCalendarDesc") },
        { icon: ChartLine, t: "Analitik Drip", d: t("landing.dashAnalyticsDesc") },
        { icon: MessagesSquare, t: "Live Chat", d: t("landing.dashLiveChatDesc") },
      ],
    },
    {
      id: "dev",
      label: "Developer",
      items: [
        { icon: Code2, t: "REST API", d: t("landing.devRestApiDesc") },
        { icon: KeyRound, t: "API Key", d: t("landing.devApiKeyDesc") },
        { icon: Webhook, t: "Webhook + Retry", d: t("landing.devWebhookDesc") },
        { icon: BookOpen, t: t("landing.devDocsTitle"), d: t("landing.devDocsDesc") },
        { icon: ShieldCheck, t: t("landing.devSecurityTitle"), d: t("landing.devSecurityDesc") },
        { icon: Zap, t: t("landing.devSpeedTitle"), d: t("landing.devSpeedDesc") },
      ],
    },
  ];
  const active = fiturTabs.find((x) => x.id === tab)!;
  return (
    <section id="fitur" className="py-20 bg-card scroll-mt-16">
      <div className="max-w-6xl mx-auto px-4">
        <SectionHead kicker="Fitur" title="Fitur-fitur SlackerSender" desc="Semua yang Anda butuhkan untuk otomatisasi WhatsApp dalam satu platform." />
        <Reveal className="flex flex-wrap justify-center gap-2 mb-10">
          {fiturTabs.map((ft) => (
            <button
              key={ft.id}
              onClick={() => setTab(ft.id)}
              className={`px-5 py-2.5 rounded-full text-sm font-semibold transition-all ${
                tab === ft.id ? "bg-[#15803d] text-white shadow-lg shadow-[#15803d]/25" : "bg-background text-muted-foreground border border-border hover:border-[#2fb35d]"
              }`}
            >
              {ft.label}
            </button>
          ))}
        </Reveal>
        <div key={tab} className="grid sm:grid-cols-2 lg:grid-cols-3 gap-5">
          {active.items.map((it, i) => (
            <Reveal key={it.t} delay={Math.min(i, 5) * 60}>
              <div className="h-full bg-background border border-border rounded-2xl p-6 hover:shadow-xl hover:shadow-slate-200/60 hover:-translate-y-1 transition-all">
                <span className="w-11 h-11 rounded-xl bg-[#15803d]/10 flex items-center justify-center mb-4">
                  <it.icon className="w-5 h-5 text-[#4ade80]" />
                </span>
                <h3 className="font-bold text-foreground mb-1.5">{it.t}</h3>
                <p className="text-sm text-muted-foreground">{it.d}</p>
              </div>
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}

/* ── Demo simulasi ────────────────────────────────── */
function DemoSimulasi() {
  const { t } = useLang();
  const [nomor, setNomor] = useState("6281234567890");
  const [pesan, setPesan] = useState("Halo! Pesanan Anda sudah dikirim 🚚");
  const [resp, setResp] = useState<string | null>(null);
  const [running, setRunning] = useState(false);

  const curlCmd = `curl -X POST ${window.location.origin}/api/messages/send \\\n  -H "X-API-Key: ••••" \\\n  -H "Content-Type: application/json" \\\n  -d '${JSON.stringify({ to: nomor.trim() || "62812...", message: pesan })}'`;

  const kirim = () => {
    const msgId = "3EB0" + Array.from({ length: 8 }, () => "0123456789ABCDEF"[Math.floor(Math.random() * 16)]).join("");
    const full = JSON.stringify(
      { status: "sent", messageId: msgId, to: nomor.trim() || "62812...", timestamp: new Date().toISOString() },
      null,
      2
    );
    setResp("");
    setRunning(true);
    let i = 0;
    const timer = setInterval(() => {
      i += 4;
      setResp(full.slice(0, i));
      if (i >= full.length) {
        clearInterval(timer);
        setRunning(false);
      }
    }, 10);
  };

  return (
    <section id="demo" className="py-20 bg-card scroll-mt-16">
      <div className="max-w-6xl mx-auto px-4">
        <SectionHead kicker={t("landing.demoKicker")} title={t("landing.demoTitle")} desc={t("landing.demoDesc")} />
        <div className="flex justify-center mb-10 -mt-6">
          <span className="inline-flex items-center gap-2 bg-amber-950/40 border border-amber-800/60 text-amber-400 text-xs font-semibold px-3 py-1.5 rounded-full">
            <FlaskConical className="w-3.5 h-3.5" /> {t("landing.demoBadge")}
          </span>
        </div>
        <div className="grid md:grid-cols-2 gap-6 items-start">
          <Reveal>
            <div className="bg-background border border-border rounded-2xl p-6">
              <label htmlFor="demo-phone" className="block text-sm font-semibold text-foreground mb-2">{t("landing.demoPhoneLabel")}</label>
              <input
                id="demo-phone"
                value={nomor}
                onChange={(e) => setNomor(e.target.value)}
                placeholder="6281234567890"
                inputMode="tel"
                className="w-full border border-border rounded-xl px-4 py-2.5 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-[#15803d] focus:border-[#15803d] mb-4"
              />
              <label htmlFor="demo-message" className="block text-sm font-semibold text-foreground mb-2">{t("landing.demoMessageLabel")}</label>
              <textarea
                id="demo-message"
                value={pesan}
                onChange={(e) => setPesan(e.target.value)}
                rows={4}
                className="w-full border border-border rounded-xl px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#15803d] focus:border-[#15803d] mb-4 resize-y"
              />
              <button
                onClick={kirim}
                disabled={running}
                className="inline-flex items-center gap-2 bg-[#15803d] hover:bg-[#0b3d1f] disabled:opacity-60 text-white font-semibold px-6 py-3 rounded-xl transition-colors"
              >
                <Play className="w-4 h-4" /> {running ? t("landing.demoSending") : t("landing.demoSendSim")}
              </button>
            </div>
          </Reveal>
          <div className="space-y-4">
            <Reveal delay={100}>
              <div className="bg-[#020402] rounded-2xl overflow-hidden">
                <div className="px-5 py-3 border-b border-white/10 text-xs text-muted-foreground font-mono">{t("landing.demoRequest")}</div>
                <pre className="p-5 text-[12.5px] font-mono leading-relaxed overflow-x-auto">
                  <code className="text-muted-foreground whitespace-pre-wrap break-all">{curlCmd}</code>
                </pre>
              </div>
            </Reveal>
            <Reveal delay={150}>
              <div className="bg-[#020402] rounded-2xl overflow-hidden min-h-[190px]">
                <div className="px-5 py-3 border-b border-white/10 flex items-center justify-between">
                  <span className="text-xs text-muted-foreground font-mono">{t("landing.demoResponse")}</span>
                  {resp !== null && !running && <span className="text-xs font-mono font-bold text-[#15803d]">200 OK</span>}
                </div>
                <pre className="p-5 text-[12.5px] font-mono leading-relaxed overflow-x-auto">
                  <code className="text-[#2fb35d] whitespace-pre-wrap">
                    {resp === null ? <span className="text-muted-foreground">{t("landing.demoResponseHint").replace("{action}", t("landing.demoSendSim"))}</span> : resp + (running ? "▍" : "")}
                  </code>
                </pre>
              </div>
            </Reveal>
          </div>
        </div>
      </div>
    </section>
  );
}

/* ── Integrasi ──────────────────────────────────────── */
function Integrasi() {
  const { t } = useLang();
  const items = [t("landing.integration1"), "CRM", "ERP", t("landing.integration4"), t("landing.integration5"), t("landing.integration6"), "E-Commerce", "Helpdesk"];
  return (
    <section className="py-16 bg-[#061006] border-y border-border overflow-hidden">
      <div className="max-w-6xl mx-auto px-4 text-center mb-8">
        <Reveal>
          <h2 className="text-2xl md:text-3xl font-extrabold text-white">{t("landing.integrationsTitle")}</h2>
          <p className="text-muted-foreground mt-2">{t("landing.integrationsDesc")}</p>
        </Reveal>
      </div>
      <div className="relative">
        <div className="flex gap-4 w-max marquee">
          {[...items, ...items].map((it, i) => (
            <span key={i} className="bg-background/10 border border-white/20 text-white text-sm font-semibold px-5 py-2.5 rounded-full whitespace-nowrap backdrop-blur">
              {it}
            </span>
          ))}
        </div>
        <style>{`@keyframes marquee { from { transform: translateX(0); } to { transform: translateX(-50%); } } .marquee { animation: marquee 22s linear infinite; }`}</style>
      </div>
    </section>
  );
}

/* ── Testimoni ──────────────────────────────────────── */
function Testimoni() {
  const { t } = useLang();
  const data = [
    { n: "Rian", r: t("landing.testiRole1"), t: t("landing.testi1") },
    { n: "Sinta", r: "Digital Marketer", t: t("landing.testi2") },
    { n: "Budi", r: "Developer", t: t("landing.testi3") },
    { n: "Maya", r: t("landing.testiRole4"), t: t("landing.testi4") },
    { n: "Andi", r: t("landing.testiRole5"), t: t("landing.testi5") },
    { n: "Dewi", r: "HR Startup", t: t("landing.testi6") },
  ];
  return (
    <section className="py-20 bg-background">
      <div className="max-w-6xl mx-auto px-4">
        <SectionHead kicker={t("landing.testimonialKicker")} title={t("landing.testimonialTitle")} />
        <div className="grid md:grid-cols-3 gap-5">
          {data.map((x, i) => (
            <Reveal key={x.n} delay={(i % 3) * 80}>
              <div className="h-full bg-card border border-border rounded-2xl p-6 flex flex-col">
                <div className="flex gap-0.5 mb-3">
                  {Array.from({ length: 5 }).map((_, s) => (
                    <Star key={s} className="w-4 h-4 fill-amber-400 text-amber-400" />
                  ))}
                </div>
                <p className="text-sm text-muted-foreground flex-1">"{x.t}"</p>
                <div className="flex items-center gap-3 mt-5">
                  <span className="w-10 h-10 rounded-full bg-[#15803d]/15 text-[#4ade80] font-bold flex items-center justify-center">{x.n[0]}</span>
                  <div>
                    <p className="text-sm font-bold text-foreground">{x.n}</p>
                    <p className="text-xs text-muted-foreground">{x.r}</p>
                  </div>
                </div>
              </div>
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}

/* ── Harga ──────────────────────────────────────────── */
type Paket = { nama: string; harga: number; pesan: string; device: number; kontak: number; bulk: number; fitur: string[]; populer?: boolean };
type PlanDB = {
  id: number | string;
  name: string;
  slug: string;
  description?: string;
  price: number;
  duration?: string;
  maxDevices: number;
  maxMessages: number;
  maxContacts?: number;
  maxBulk?: number;
  maxAutoReply?: number;
  features: string[];
};

function kuotaLabel(t: (k: string) => string, maxMessages: number): string {
  if (!maxMessages || maxMessages >= 1000000) return t("landing.unlimitedQuota");
  return t("landing.quotaPerMonth").replace("{count}", maxMessages.toLocaleString("id-ID"));
}

function Harga() {
  const { t } = useLang();
  const [tahunan, setTahunan] = useState(false);
  const [plans, setPlans] = useState<PlanDB[] | null>(null);
  useEffect(() => {
    let alive = true;
    fetch("/api/public/plans")
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => {
        if (alive && d && Array.isArray(d.plans) && d.plans.length > 0) setPlans(d.plans);
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);
  const paketBulanan: Paket[] = [
    {
      nama: "Free",
      harga: 0,
      pesan: t("landing.quotaPerMonth").replace("{count}", "1.000"),
      device: 1,
      kontak: 500,
      bulk: 100,
      fitur: [t("landing.freeFeat1"), t("landing.freeFeat2"), t("landing.freeFeat3"), t("landing.freeFeat4"), "Autoreply keyword", "Webhook", "REST API"],
    },
    {
      nama: "Lite",
      harga: 25000,
      pesan: t("landing.quotaPerMonth").replace("{count}", "1.000"),
      device: 2,
      kontak: 1000,
      bulk: 250,
      fitur: [t("landing.liteFeat1"), t("landing.liteFeat2"), t("landing.liteFeat3"), t("landing.liteFeat4"), t("landing.liteFeat5"), t("landing.liteFeat6")],
    },
    {
      nama: "Regular",
      harga: 66000,
      pesan: t("landing.quotaPerMonth").replace("{count}", "10.000"),
      device: 3,
      kontak: 2500,
      bulk: 500,
      populer: true,
      fitur: [t("landing.regFeat1"), t("landing.regFeat2"), t("landing.regFeat3"), t("landing.regFeat4"), t("landing.regFeat5"), t("landing.regFeat6")],
    },
    {
      nama: "Pro",
      harga: 110000,
      pesan: t("landing.quotaPerMonth").replace("{count}", "25.000"),
      device: 5,
      kontak: 5000,
      bulk: 750,
      fitur: [t("landing.proFeat1"), t("landing.proFeat2"), t("landing.proFeat3"), t("landing.proFeat4"), t("landing.proFeat5")],
    },
    {
      nama: "Master",
      harga: 175000,
      pesan: t("landing.unlimitedQuota"),
      device: 10,
      kontak: 10000,
      bulk: 1000,
      fitur: [t("landing.masterFeat1"), t("landing.masterFeat2"), t("landing.masterFeat3"), t("landing.masterFeat4"), t("landing.masterFeat5")],
    },
  ];
  const rp = (n: number) => (n === 0 ? "Rp 0" : "Rp " + Math.round(n).toLocaleString("id-ID"));
  const daftar: Paket[] = plans
    ? plans.map((p) => ({
        nama: p.name,
        harga: p.price,
        pesan: kuotaLabel(t, p.maxMessages),
        device: p.maxDevices,
        kontak: p.maxContacts ?? 0,
        bulk: p.maxBulk ?? 0,
        fitur: Array.isArray(p.features) ? p.features : [],
        populer: p.slug === "regular" || p.name.toLowerCase() === "regular",
      }))
    : paketBulanan;
  return (
    <section id="harga" className="py-20 bg-card scroll-mt-16">
      <div className="max-w-7xl mx-auto px-4">
        <SectionHead kicker={t("landing.pricingKicker")} title={t("landing.pricingTitle")} desc={t("landing.pricingDesc")} />
        <Reveal className="flex justify-center mb-10">
          <div className="bg-background border border-border rounded-full p-1 flex text-sm font-semibold">
            <button onClick={() => setTahunan(false)} className={`px-5 py-2 rounded-full transition-all ${!tahunan ? "bg-[#15803d] text-white shadow" : "text-muted-foreground"}`}>
              {t("landing.monthly")}
            </button>
            <button onClick={() => setTahunan(true)} className={`px-5 py-2 rounded-full transition-all ${tahunan ? "bg-[#15803d] text-white shadow" : "text-muted-foreground"}`}>
              {t("landing.yearly")}
            </button>
          </div>
        </Reveal>
        <div className="grid sm:grid-cols-2 lg:grid-cols-5 gap-5">
          {daftar.map((p, i) => {
            const harga = tahunan ? p.harga * 10 : p.harga;
            return (
              <Reveal key={p.nama} delay={i * 60}>
                <div className={`relative h-full rounded-2xl p-6 flex flex-col bg-background ${p.populer ? "border-2 border-[#15803d] shadow-xl" : "border border-border"}`}>
                  {p.populer && (
                    <span className="absolute -top-3 left-1/2 -translate-x-1/2 bg-[#15803d] text-white text-xs font-bold px-3 py-1 rounded-full">{t("landing.bestSeller")}</span>
                  )}
                  <h3 className="font-extrabold text-lg text-foreground text-center">{p.nama}</h3>
                  <p className="text-3xl font-extrabold mt-2 text-foreground text-center">{rp(harga)}</p>
                  <p className="text-xs mt-1 text-muted-foreground text-center">{tahunan ? t("landing.perYear") : t("landing.perMonth")}</p>
                  <p className="text-xs font-semibold mt-4 text-[#4ade80] text-center">
                    {p.pesan} · {p.device} device · {t("landing.contactsCount").replace("{count}", p.kontak.toLocaleString("id-ID"))} · {t("landing.bulkPerBlast").replace("{count}", p.bulk.toLocaleString("id-ID"))}
                  </p>
                  <hr className="my-4 border-border" />
                  <ul className="space-y-2 flex-1">
                    {p.fitur.map((f) => (
                      <li key={f} className="text-[13px] flex gap-2 text-muted-foreground">
                        <Check className="w-4 h-4 text-[#15803d] shrink-0 mt-0.5" /> {f}
                      </li>
                    ))}
                  </ul>
                  <Link href="/register" className="mt-6 text-center text-sm font-semibold rounded-xl px-4 py-2.5 transition-colors bg-[#15803d] hover:bg-[#0b3d1f] text-white">
                    {p.harga === 0 ? t("landing.startFree") : t("landing.choosePlan")}
                  </Link>
                </div>
              </Reveal>
            );
          })}
        </div>
        <Reveal className="text-center mt-8">
          <p className="text-sm text-muted-foreground">{t("landing.pricingNote1")} <b>{t("landing.pricingNoteBold")}</b> {t("landing.pricingNote2")}</p>
        </Reveal>
      </div>
    </section>
  );
}

/* ── FAQ ────────────────────────────────────────────── */
function FAQ() {
  const { t } = useLang();
  const [open, setOpen] = useState<number | null>(0);
  const items = [
    { q: t("landing.faq1Q"), a: t("landing.faq1A") },
    { q: t("landing.faq2Q"), a: t("landing.faq2A") },
    { q: t("landing.faq3Q"), a: t("landing.faq3A") },
    { q: t("landing.faq4Q"), a: t("landing.faq4A") },
    { q: t("landing.faq5Q"), a: t("landing.faq5A") },
    { q: t("landing.faq6Q"), a: t("landing.faq6A") },
    { q: t("landing.faq7Q"), a: t("landing.faq7A") },
    { q: t("landing.faq8Q"), a: t("landing.faq8A") },
  ];
  return (
    <section id="faq" className="py-20 bg-background scroll-mt-16">
      <div className="max-w-3xl mx-auto px-4">
        <SectionHead kicker="FAQ" title={t("landing.faqTitle")} />
        <div className="space-y-3">
          {items.map((it, i) => (
            <Reveal key={i} delay={Math.min(i, 4) * 50}>
              <div className={`border rounded-2xl overflow-hidden transition-colors ${open === i ? "border-[#2fb35d] bg-[#0d1c10]/40" : "border-border bg-background"}`}>
                <button onClick={() => setOpen(open === i ? null : i)} className="w-full flex items-center justify-between gap-4 px-5 py-4 text-left">
                  <span className="font-semibold text-foreground text-[15px]">{it.q}</span>
                  <ChevronDown className={`w-5 h-5 text-[#4ade80] shrink-0 transition-transform ${open === i ? "rotate-180" : ""}`} />
                </button>
                <div className={`grid transition-all duration-300 ${open === i ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0"}`}>
                  <div className="overflow-hidden">
                    <p className="px-5 pb-5 text-sm text-muted-foreground leading-relaxed">{it.a}</p>
                  </div>
                </div>
              </div>
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}

/* ── CTA + Footer ───────────────────────────────────── */
function CTA() {
  const { t } = useLang();
  return (
    <section className="relative py-20 overflow-hidden bg-background">
      <div aria-hidden className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[900px] max-w-[170vw] aspect-square bg-muted" style={{ borderRadius: "42% 58% 61% 39% / 45% 42% 58% 55%" }} />
      <div className="relative max-w-3xl mx-auto px-4 text-center">
        <Reveal>
          <h2 className="text-3xl md:text-4xl font-black text-[#4ade80] tracking-tight">{t("landing.ctaTitle")}</h2>
          <p className="text-muted-foreground mt-4">{t("landing.ctaDesc")}</p>
          <div className="flex flex-wrap justify-center gap-3 mt-8">
            <Link href="/register" className="inline-flex items-center gap-2 bg-[#15803d] hover:bg-[#15803d] text-white font-semibold px-8 py-3.5 rounded-xl transition-colors shadow-lg shadow-[#15803d]/25">
              <Send className="w-4 h-4" /> {t("landing.ctaTryNow")}
            </Link>
            <Link href="/api-docs" className="inline-flex items-center gap-2 border border-[#0b3d1f]/20 hover:border-[#15803d] text-[#4ade80] font-semibold px-8 py-3.5 rounded-xl transition-colors bg-background/60">
              {t("landing.ctaReadDocs")}
            </Link>
          </div>
        </Reveal>
      </div>
    </section>
  );
}

function Footer() {
  const { t } = useLang();
  return (
    <footer className="bg-[#061006] text-white">
      <div className="max-w-6xl mx-auto px-4 py-12">
        <div className="grid md:grid-cols-4 gap-8">
          <div>
            <div className="flex items-center gap-2 mb-4">
              <img src="/slackercode-logo.png" alt="" className="h-9 w-9" />
              <span className="font-extrabold text-xl text-white">SlackerSender</span>
            </div>
            <p className="text-sm text-white/90 leading-relaxed">
              {t("landing.footerDisclaimer")}
            </p>
          </div>
          <div>
            <h2 className="text-lg font-bold text-white mb-4">{t("landing.footerInfo")}</h2>
            <div className="space-y-2.5 text-sm">
              <a href="#fitur" className="flex items-center gap-1 text-white hover:text-white/70"><ChevronRight className="w-3.5 h-3.5" /> {t("landing.navFeatures")}</a>
              <a href="#harga" className="flex items-center gap-1 text-white hover:text-white/70"><ChevronRight className="w-3.5 h-3.5" /> {t("landing.navPricing")}</a>
              <a href="#faq" className="flex items-center gap-1 text-white hover:text-white/70"><ChevronRight className="w-3.5 h-3.5" /> {t("landing.navFaq")}</a>
              <Link href="/api-docs" className="flex items-center gap-1 text-white hover:text-white/70"><ChevronRight className="w-3.5 h-3.5" /> {t("landing.docs")}</Link>
              <a href="#demo" className="flex items-center gap-1 text-white hover:text-white/70"><ChevronRight className="w-3.5 h-3.5" /> {t("landing.demoKicker")}</a>
            </div>
          </div>
          <div>
            <h2 className="text-lg font-bold text-white mb-4">{t("landing.footerAbout")}</h2>
            <div className="space-y-2.5 text-sm">
              <a href="#" className="flex items-center gap-1 text-white hover:text-white/70"><ChevronRight className="w-3.5 h-3.5" /> Disclaimer</a>
              <a href="#" className="flex items-center gap-1 text-white hover:text-white/70"><ChevronRight className="w-3.5 h-3.5" /> Terms &amp; Conditions</a>
            </div>
          </div>
          <div>
            <h2 className="text-lg font-bold text-white mb-4">{t("landing.footerContact")}</h2>
            <div className="space-y-2.5 text-sm">
              <p className="flex items-start gap-2 text-white"><MapPin className="w-4 h-4 mt-0.5 shrink-0" /> {t("landing.footerLocation")}</p>
              <a href="#" className="flex items-center gap-2 text-white hover:text-white/70"><CircleHelp className="w-4 h-4 shrink-0" /> Support</a>
            </div>
          </div>
        </div>
      </div>
      <div className="bg-[#020402]">
        <p className="max-w-6xl mx-auto px-4 py-4 text-center text-sm text-white">
          {t("landing.footerMadeWith1")} <span className="text-red-500">❤</span> {t("landing.footerMadeWith2")}
        </p>
        <img src="/partner-slackercode.png" alt="Slackercode Family" className="mx-auto mt-3 h-5 w-auto opacity-80" />
      </div>
    </footer>
  );
}

/* ── Sticky CTA (mobile) ────────────────────────────── */
function StickyCTA() {
  const { t } = useLang();
  const [show, setShow] = useState(false);
  useEffect(() => {
    const fn = () => {
      const y = window.scrollY;
      const nearBottom = window.innerHeight + y > document.documentElement.scrollHeight - 200;
      setShow(y > 600 && !nearBottom);
    };
    window.addEventListener("scroll", fn, { passive: true });
    fn();
    return () => window.removeEventListener("scroll", fn);
  }, []);
  return (
    <div
      aria-hidden={!show}
      className={`md:hidden fixed bottom-0 inset-x-0 z-40 transition-transform duration-300 ${show ? "translate-y-0 visible" : "translate-y-full invisible"}`}
    >
      <div className="bg-background border-t border-border shadow-[0_-4px_20px_rgba(0,0,0,0.08)] px-4 py-3 flex items-center justify-between gap-3">
        <p className="text-sm font-semibold text-foreground">{t("landing.stickyTitle")}</p>
        <Link href="/register" className="bg-[#15803d] hover:bg-[#0b3d1f] text-white text-sm font-semibold px-5 py-2.5 rounded-xl transition-colors whitespace-nowrap">
          {t("landing.tryFree")}
        </Link>
      </div>
    </div>
  );
}

export default function Landing() {
  return (
    <div className="min-h-screen bg-background font-sans antialiased">
      <Navbar />
      <main>
        <Hero />
        <Kelebihan />
        <Fitur />
        <DemoSimulasi />
        <Integrasi />
        <Testimoni />
        <Harga />
        <FAQ />
        <CTA />
      </main>
      <Footer />
      <StickyCTA />
    </div>
  );
}
