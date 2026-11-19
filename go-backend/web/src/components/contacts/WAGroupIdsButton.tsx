/*
 * Tombol + dialog "ID Grup": menampilkan ID (JID) grup WhatsApp yang diikuti
 * perangkat terhubung, lengkap dengan tombol salin. ID ini dipakai sebagai
 * penerima di API (field `number`, contoh 120363181931716012@g.us).
 *
 * Author: Mas Wondho — Duta Corpora Indonesia (DCI)
 * Copyright © 2026 Duta Corpora Indonesia
 */
import { useEffect, useMemo, useState } from "react";
import { Copy, Hash, Loader2, X } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dropdown } from "@/components/ui/dropdown";
import { apiGet } from "@/lib/api";
import { useLang } from "@/lib/i18n";

interface Device {
  id: number;
  name: string;
  phone: string;
  status: string;
}

interface WAGroup {
  jid: string;
  name: string;
  participantCount: number;
}

export default function WAGroupIdsButton() {
  const { t } = useLang();
  const [open, setOpen] = useState(false);
  const [devices, setDevices] = useState<Device[]>([]);
  const [deviceId, setDeviceId] = useState("");
  const [groups, setGroups] = useState<WAGroup[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState("");

  // Buka dialog: ambil perangkat terhubung, pilih yang pertama.
  const openDialog = async () => {
    setOpen(true);
    setError(null);
    setGroups([]);
    setFilter("");
    try {
      const res = await apiGet<{ devices: Device[] }>("/devices");
      const online = (res.devices || []).filter((d) => d.status === "connected");
      setDevices(online);
      if (online.length === 0) {
        setDeviceId("");
        setError(t("waGroupIds.errNoDevices"));
        return;
      }
      setDeviceId(String(online[0].id));
    } catch (e) {
      setError(e instanceof Error ? e.message : t("waGroupIds.errLoadDevices"));
    }
  };

  // Muat grup setiap perangkat berganti. Flag `batal` mencegah hasil lama
  // menimpa hasil perangkat yang baru dipilih.
  useEffect(() => {
    if (!open || !deviceId) return;
    let batal = false;
    setLoading(true);
    setError(null);
    apiGet<{ groups: WAGroup[] }>(`/groups?deviceId=${encodeURIComponent(deviceId)}`)
      .then((res) => {
        if (!batal) setGroups(res.groups || []);
      })
      .catch((e) => {
        if (!batal) {
          setGroups([]);
          setError(e instanceof Error ? e.message : t("waGroupIds.errLoadGroups"));
        }
      })
      .finally(() => {
        if (!batal) setLoading(false);
      });
    return () => {
      batal = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, deviceId]);

  // Escape menutup dialog.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  const shown = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return groups;
    return groups.filter((g) => g.name.toLowerCase().includes(q) || g.jid.toLowerCase().includes(q));
  }, [groups, filter]);

  const copy = async (jid: string) => {
    try {
      await navigator.clipboard.writeText(jid);
      toast.success(t("waGroupIds.copied"));
    } catch {
      toast.error(t("waGroupIds.errCopy"));
    }
  };

  return (
    <>
      <Button variant="outline" onClick={openDialog} className="gap-1.5">
        <Hash className="w-4 h-4" />
        {t("waGroupIds.buttonLabel")}
      </Button>

      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={() => setOpen(false)}>
          <div
            role="dialog"
            aria-modal="true"
            aria-labelledby="wa-group-ids-title"
            className="w-full max-w-2xl max-h-[90vh] flex flex-col rounded-xl bg-card text-card-foreground border border-border p-5 sm:p-6 shadow-xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-start justify-between gap-3 mb-1">
              <h3 id="wa-group-ids-title" className="text-lg font-semibold">
                {t("waGroupIds.title")}
              </h3>
              <button onClick={() => setOpen(false)} className="p-1.5 rounded-md hover:bg-secondary" aria-label={t("common.close")}>
                <X className="w-5 h-5" />
              </button>
            </div>
            <p className="text-sm text-muted-foreground mb-4">{t("waGroupIds.desc")}</p>

            {devices.length > 1 && (
              <div className="mb-3">
                <Dropdown
                  value={deviceId}
                  onChange={setDeviceId}
                  ariaLabel={t("waGroupIds.pickDevice")}
                  options={devices.map((d) => ({ value: String(d.id), label: `${d.name} (${d.phone || d.id})` }))}
                />
              </div>
            )}

            {groups.length > 0 && (
              <Input
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                placeholder={t("waGroupIds.search")}
                aria-label={t("waGroupIds.search")}
                className="mb-3"
              />
            )}

            <div className="flex-1 overflow-y-auto space-y-2 min-h-[6rem]" aria-live="polite">
              {loading ? (
                <div className="flex items-center justify-center gap-2 py-8 text-sm text-muted-foreground">
                  <Loader2 className="w-4 h-4 animate-spin" />
                  {t("waGroupIds.loading")}
                </div>
              ) : error ? (
                <p className="py-6 text-center text-sm text-destructive">{error}</p>
              ) : deviceId && groups.length === 0 ? (
                <p className="py-6 text-center text-sm text-muted-foreground">{t("waGroupIds.empty")}</p>
              ) : shown.length === 0 && groups.length > 0 ? (
                <p className="py-6 text-center text-sm text-muted-foreground">{t("waGroupIds.noMatch")}</p>
              ) : (
                shown.map((g) => (
                  <div key={g.jid} className="flex items-center gap-3 rounded-lg border border-border px-3 py-2.5">
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium">{g.name || t("waGroupIds.noName")}</p>
                      <p className="truncate font-mono text-xs text-muted-foreground select-all">{g.jid}</p>
                    </div>
                    <span className="hidden sm:block shrink-0 text-xs text-muted-foreground">
                      {t("waGroupIds.members").replace("{n}", String(g.participantCount))}
                    </span>
                    <Button variant="outline" size="sm" onClick={() => copy(g.jid)} className="shrink-0 gap-1.5" aria-label={`${t("waGroupIds.copy")} ${g.name}`}>
                      <Copy className="w-3.5 h-3.5" />
                      {t("waGroupIds.copy")}
                    </Button>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      )}
    </>
  );
}
