import { useEffect, useState } from "react";
import { toast } from "sonner";
import { History as HistoryIcon, Undo2, ChevronLeft, ChevronRight, Eye } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { apiGet, apiDelete } from "@/lib/api";
import { useLang } from "@/lib/i18n";
import { MessagePreviewDialog } from "@/components/history/MessagePreviewDialog";

interface HistoryMessage {
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

interface Device {
  id: number;
  name: string;
  phone: string;
}

function statusBadge(status: string, t: (key: string) => string) {
  const s = (status || "").toLowerCase();
  if (s === "read") return <Badge variant="success">{t("history.statusRead")}</Badge>;
  if (s === "delivered") return <Badge variant="default">{t("history.statusDelivered")}</Badge>;
  if (s === "sent") return <Badge variant="default">{t("history.statusDelivered")}</Badge>;
  if (s === "failed") return <Badge variant="destructive">{t("history.statusFailed")}</Badge>;
  if (s === "revoked") return <Badge variant="secondary">{t("history.statusRevoked")}</Badge>;
  if (s === "pending") return <Badge variant="secondary">{t("history.statusPending")}</Badge>;
  return <Badge variant="secondary">{status || "-"}</Badge>;
}

function formatDate(iso: string) {
  if (!iso) return "-";
  return new Date(iso).toLocaleString("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

const PAGE_SIZE = 20;

const HEADERS = [
  "headerId",
  "headerSender",
  "headerNumber",
  "headerMessage",
  "headerStatus",
  "headerVia",
  "headerDate",
  "headerAction",
];

export default function History() {
  const { t } = useLang();
  const [rows, setRows] = useState<HistoryMessage[]>([]);
  const [devices, setDevices] = useState<Record<number, string>>({});
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [revoking, setRevoking] = useState<number | null>(null);
  const [previewIdx, setPreviewIdx] = useState<number | null>(null);

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const load = async (p: number) => {
    setLoading(true);
    try {
      const [m, d] = await Promise.all([
        apiGet<{ messages: HistoryMessage[]; total: number }>(`/messages?page=${p}&limit=${PAGE_SIZE}`),
        apiGet<{ devices: Device[] }>("/devices"),
      ]);
      setRows(m.messages ?? []);
      setTotal(m.total ?? 0);
      const map: Record<number, string> = {};
      for (const dev of d.devices ?? []) map[dev.id] = dev.phone || dev.name || `#${dev.id}`;
      setDevices(map);
      setPage(p);
      setPreviewIdx(null);
    } catch (e: any) {
      toast.error(e.message || t("history.loadFailed"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load(1);
  }, []);

  const canRevoke = (m: HistoryMessage) =>
    ["sent", "delivered", "read"].includes((m.status || "").toLowerCase()) && !!m.messageId;

  const revoke = async (m: HistoryMessage) => {
    if (!window.confirm(t("history.revokeConfirm").replace("{to}", m.to))) return;
    setRevoking(m.id);
    try {
      await apiDelete(`/messages/${m.id}`);
      toast.success(t("history.revoked"));
      setRows((rs) => rs.map((r) => (r.id === m.id ? { ...r, status: "revoked" } : r)));
    } catch (e: any) {
      toast.error(e.message || t("history.revokeFailed"));
    } finally {
      setRevoking(null);
    }
  };

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold text-foreground">{t("history.title")}</h1>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <HistoryIcon className="w-4 h-4" /> {t("history.allMessages")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="space-y-2">
              {[0, 1, 2, 3].map((i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          ) : rows.length === 0 ? (
            <EmptyState
              icon={HistoryIcon}
              title={t("history.emptyTitle")}
              hint={t("history.emptyHint")}
            />
          ) : (
            <>
              <div className="overflow-x-auto">
                <table className="w-full min-w-[860px] text-sm">
                  <thead>
                    <tr className="border-b border-border text-left">
                      {HEADERS.map((h) => (
                        <th key={h} className="py-2.5 pr-4 text-[11px] font-medium uppercase tracking-wide text-muted-foreground last:pr-0">
                          {t(`history.${h}`)}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((m) => (
                      <tr
                        key={m.id}
                        className="border-b border-border last:border-0 cursor-pointer hover:bg-secondary/40"
                        onClick={() => setPreviewIdx(rows.indexOf(m))}
                      >
                        <td className="py-3 pr-4 font-mono text-[13px]">{m.id}</td>
                        <td className="py-3 pr-4 font-mono text-[13px]">{devices[m.deviceId] ?? `#${m.deviceId}`}</td>
                        <td className="py-3 pr-4 font-mono text-[13px]">{m.to}</td>
                        <td className="py-3 pr-4 text-xs text-muted-foreground max-w-[280px] truncate">
                          <button
                            type="button"
                            className="max-w-full truncate text-left hover:text-foreground hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm"
                            title={t("history.previewOpen")}
                            onClick={(e) => {
                              e.stopPropagation();
                              setPreviewIdx(rows.indexOf(m));
                            }}
                          >
                            {m.content || m.caption || (m.type && m.type !== "text" ? `[${m.type}]` : "-")}
                          </button>
                        </td>
                        <td className="py-3 pr-4">{statusBadge(m.status, t)}</td>
                        <td className="py-3 pr-4">
                          <Badge variant="secondary">{m.via || "-"}</Badge>
                        </td>
                        <td className="py-3 pr-4 text-xs text-muted-foreground whitespace-nowrap">{formatDate(m.createdAt)}</td>
                        <td className="py-3 whitespace-nowrap" onClick={(e) => e.stopPropagation()}>
                          <Button
                            variant="secondary"
                            size="sm"
                            className="mr-1.5"
                            aria-label={t("history.previewOpen")}
                            title={t("history.previewOpen")}
                            onClick={() => setPreviewIdx(rows.indexOf(m))}
                          >
                            <Eye className="w-3.5 h-3.5" />
                          </Button>
                          {canRevoke(m) ? (
                            <Button
                              variant="destructive"
                              size="sm"
                              disabled={revoking === m.id}
                              onClick={() => revoke(m)}
                            >
                              <Undo2 className="w-3.5 h-3.5 mr-1" />
                              {revoking === m.id ? "..." : t("history.revokeButton")}
                            </Button>
                          ) : null}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <div className="mt-4 flex items-center justify-between">
                <p className="text-xs text-muted-foreground">
                  {t("history.pageInfo")
                    .replace("{page}", String(page))
                    .replace("{totalPages}", String(totalPages))
                    .replace("{total}", total.toLocaleString("id-ID"))}
                </p>
                <div className="flex gap-2">
                  <Button variant="secondary" size="sm" disabled={page <= 1 || loading} onClick={() => load(page - 1)}>
                    <ChevronLeft className="w-4 h-4" />
                  </Button>
                  <Button variant="secondary" size="sm" disabled={page >= totalPages || loading} onClick={() => load(page + 1)}>
                    <ChevronRight className="w-4 h-4" />
                  </Button>
                </div>
              </div>
            </>
          )}
        </CardContent>
      </Card>

      {previewIdx !== null && rows[previewIdx] && (
        <MessagePreviewDialog
          message={rows[previewIdx]}
          deviceLabel={devices[rows[previewIdx].deviceId] ?? `#${rows[previewIdx].deviceId}`}
          hasPrev={previewIdx > 0}
          hasNext={previewIdx < rows.length - 1}
          onPrev={() => setPreviewIdx((i) => (i === null ? i : Math.max(0, i - 1)))}
          onNext={() => setPreviewIdx((i) => (i === null ? i : Math.min(rows.length - 1, i + 1)))}
          onClose={() => setPreviewIdx(null)}
        />
      )}
    </div>
  );
}
