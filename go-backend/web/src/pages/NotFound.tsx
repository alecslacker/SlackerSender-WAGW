import { Link } from "wouter";
import { Button } from "@/components/ui/button";
import { useLang } from "@/lib/i18n";

export default function NotFound() {
  const { t } = useLang();
  return (
    <div className="min-h-screen flex items-center justify-center bg-background px-4">
      <div className="text-center">
        <p className="text-6xl font-bold text-foreground">404</p>
        <p className="text-sm text-muted-foreground mt-2">
          {t("notFound.message")}
        </p>
        {/* Tombol level blok (flex) di dalam tautan inline tidak tergeser oleh text-center;
            tautan dibuat inline-block agar menyusut selebar tombol dan ikut ke tengah. */}
        <Link href="/" className="mt-6 inline-block">
          <Button variant="outline">
            {t("notFound.backToDashboard")}
          </Button>
        </Link>
      </div>
    </div>
  );
}
