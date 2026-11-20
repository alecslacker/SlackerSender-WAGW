// Kartu "Koneksi Pencarian": penyedia pencarian web terpisah dari model AI, diuji
// SEBELUM dipakai. Pola sama dengan AIConnectionCard (uji nilai form yang belum
// tersimpan; hasil disimpan server hanya untuk koneksi tersimpan).
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Eye, EyeOff, FlaskConical, Search, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { apiGet, apiPut, apiDelete, apiPost } from "@/lib/api";
import { useLang } from "@/lib/i18n";

interface SearchConnection {
  configured: boolean;
  provider?: string;
  baseUrl?: string;
  keyHint?: string;
  isActive?: boolean;
  // Hasil uji untuk penyedia TERSIMPAN ("" = belum dicek / penyedia sudah diganti).
  searchStatus?: "yes" | "no" | "";
}

interface SearchTest {
  ok: boolean;
  status: "yes" | "no" | "unknown";
  message: string;
  count?: number;
  samples?: string[];
  saved?: boolean;
  latencyMs?: number;
}

const PROVIDERS = [
  { value: "brave", label: "Brave Search" },
  { value: "zai", label: "Z.ai Coding Plan (MCP)" },
  { value: "tavily", label: "Tavily" },
  { value: "bocha", label: "BoCha (博查)" },
  { value: "querit", label: "Querit" },
  { value: "searxng", label: "SearXNG (instance sendiri)" },
];

const inputCls =
  "w-full rounded-md border border-input bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring";
const labelCls = "block text-xs font-medium text-muted-foreground mb-1";

export default function AISearchConnectionCard({ onChanged }: { onChanged?: () => void }) {
  const { t } = useLang();
  const [conn, setConn] = useState<SearchConnection | null>(null);
  const [loading, setLoading] = useState(true);
  const [provider, setProvider] = useState("brave");
  const [apiKey, setApiKey] = useState("");
  const [showKey, setShowKey] = useState(false);
  const [baseUrl, setBaseUrl] = useState("");
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [result, setResult] = useState<SearchTest | null>(null);

  const isSearx = provider === "searxng";

  const load = async () => {
    setLoading(true);
    try {
      const r = await apiGet<SearchConnection>("/ai-search");
      setConn(r);
      if (r.configured) {
        setProvider(r.provider || "brave");
        setBaseUrl(r.baseUrl || "");
      }
    } catch {
      /* biarkan kartu tampil kosong */
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const body = () => ({ provider, apiKey: apiKey.trim(), baseUrl: isSearx ? baseUrl.trim() : "" });

  const save = async () => {
    if (!conn?.configured && !isSearx && !apiKey.trim()) {
      toast.error(t("aiReply.searchApiKey") + " wajib diisi");
      return;
    }
    if (isSearx && !baseUrl.trim()) {
      toast.error(t("aiReply.searchBaseUrl") + " wajib diisi");
      return;
    }
    setSaving(true);
    setResult(null);
    try {
      const r = await apiPut<{ connection: SearchConnection }>("/ai-search", body());
      setConn(r.connection);
      setApiKey("");
      toast.success(t("aiReply.searchSaved"));
      onChanged?.();
    } catch (e: any) {
      toast.error(e.message || t("aiReply.saveError"));
    } finally {
      setSaving(false);
    }
  };

  const test = async () => {
    setTesting(true);
    setResult(null);
    try {
      const r = await apiPost<SearchTest>("/ai-search/test", body());
      setResult(r);
      if (r.saved) {
        setConn((c) => (c ? { ...c, searchStatus: r.status === "yes" ? "yes" : "no" } : c));
        onChanged?.();
      }
    } catch (e: any) {
      setResult({ ok: false, status: "unknown", message: e.message || t("aiReply.searchTestFail") });
    } finally {
      setTesting(false);
    }
  };

  const remove = async () => {
    if (!confirm(t("aiReply.searchDelete") + "?")) return;
    try {
      await apiDelete("/ai-search");
      setConn({ configured: false });
      setApiKey("");
      setResult(null);
      toast.success(t("aiReply.searchDeleted"));
      onChanged?.();
    } catch (e: any) {
      toast.error(e.message || t("aiReply.deleteError"));
    }
  };

  const providerLabel = PROVIDERS.find((p) => p.value === conn?.provider)?.label ?? conn?.provider;

  return (
    <Card>
      <CardContent className="p-4 sm:p-5">
        <div className="flex flex-wrap items-center justify-between gap-2 mb-1">
          <div className="flex items-center gap-2">
            <Search className="w-5 h-5 text-primary" />
            <h2 className="text-base font-semibold">{t("aiReply.searchTitle")}</h2>
          </div>
          {loading ? null : conn?.configured ? (
            <div className="flex flex-wrap items-center gap-1.5">
              <Badge variant="success">{providerLabel}</Badge>
              {conn.searchStatus === "yes" && <Badge variant="success">{t("aiReply.searchBadgeYes")}</Badge>}
              {conn.searchStatus === "no" && <Badge variant="destructive">{t("aiReply.searchBadgeNo")}</Badge>}
              {!conn.searchStatus && <Badge variant="secondary">{t("aiReply.searchBadgeUnchecked")}</Badge>}
            </div>
          ) : (
            <Badge variant="secondary">{t("aiReply.connNotConfigured")}</Badge>
          )}
        </div>
        <p className="text-xs text-muted-foreground mb-4">{t("aiReply.searchSubtitle")}</p>

        {loading ? (
          <p className="text-sm text-muted-foreground">{t("aiReply.loading")}</p>
        ) : (
          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <label className={labelCls}>{t("aiReply.connProvider")}</label>
              <select
                className={inputCls}
                value={provider}
                onChange={(e) => { setProvider(e.target.value); setBaseUrl(""); setResult(null); }}
              >
                {PROVIDERS.map((p) => (
                  <option key={p.value} value={p.value}>{p.label}</option>
                ))}
              </select>
            </div>
            {isSearx ? (
              <div>
                <label className={labelCls}>{t("aiReply.searchBaseUrl")}</label>
                <Input
                  className={inputCls}
                  value={baseUrl}
                  onChange={(e) => setBaseUrl(e.target.value)}
                  placeholder="https://searx.contoh.com"
                  inputMode="url"
                />
              </div>
            ) : (
              <div>
                <label className={labelCls}>{t("aiReply.searchApiKey")}</label>
                <div className="relative">
                  <Input
                    type={showKey ? "text" : "password"}
                    className={`${inputCls} pr-11`}
                    value={apiKey}
                    onChange={(e) => setApiKey(e.target.value)}
                    placeholder={
                      conn?.configured && conn.provider === provider && conn.keyHint
                        ? `${conn.keyHint} (${t("aiReply.connKeySaved")})`
                        : t("aiReply.connApiKeyPlaceholder")
                    }
                    autoComplete="off"
                  />
                  <button
                    type="button"
                    className="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-muted-foreground hover:text-foreground"
                    onClick={() => setShowKey((v) => !v)}
                    aria-label={showKey ? t("aiReply.connHideKey") : t("aiReply.connShowKey")}
                  >
                    {showKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                  </button>
                </div>
              </div>
            )}
            <p className="sm:col-span-2 text-[11px] text-muted-foreground">
              {isSearx
                ? t("aiReply.searchSearxHint")
                : provider === "zai"
                  ? t("aiReply.searchZaiHint")
                  : provider === "tavily"
                    ? t("aiReply.searchTavilyHint")
                    : provider === "bocha"
                      ? t("aiReply.searchBochaHint")
                      : provider === "querit"
                        ? t("aiReply.searchQueritHint")
                        : t("aiReply.searchKeyHint")}
            </p>
          </div>
        )}

        {!loading && (
          <div className="flex flex-wrap items-center gap-2 mt-4">
            <Button size="sm" onClick={save} disabled={saving}>
              {saving ? t("aiReply.saving") : t("aiReply.connSave")}
            </Button>
            <Button size="sm" variant="tint" onClick={test} disabled={testing} className="gap-1.5">
              <FlaskConical className="w-4 h-4" />
              {testing ? t("aiReply.searchTesting") : t("aiReply.searchTest")}
            </Button>
            {conn?.configured && (
              <Button size="sm" variant="ghost" onClick={remove} className="gap-1.5 text-destructive">
                <Trash2 className="w-4 h-4" /> {t("aiReply.searchDelete")}
              </Button>
            )}
            {result && (
              <span
                className={`text-xs ${
                  result.status === "yes" ? "text-green-600"
                  : result.status === "no" ? "text-destructive"
                  : "text-amber-500"
                }`}
              >
                {result.status === "yes" &&
                  `${t("aiReply.searchOk")}${result.count != null ? ` • ${result.count} ${t("aiReply.searchResults")}` : ""}${
                    result.latencyMs != null ? ` • ${result.latencyMs}ms` : ""}`}
                {result.status !== "yes" && `${result.status === "no" ? t("aiReply.searchNo") : t("aiReply.searchUnknown")}: ${result.message}`}
                {result.status !== "unknown" && !result.saved && ` (${t("aiReply.visionNotSaved")})`}
              </span>
            )}
          </div>
        )}
        {result?.status === "yes" && result.samples && result.samples.length > 0 && (
          <ul className="mt-2 text-[11px] text-muted-foreground list-disc pl-4">
            {result.samples.map((s, i) => <li key={i}>{s}</li>)}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
