import { useEffect, useState } from "react";
import { cn } from "@/lib/utils";

// Animasi loading SlackerSender — tiga titik navy memantul (CSS murni,
// pilihan user 2026-10-01 menggantikan paper-plane Lottie).
export function PageTransitionLoader({ className }: { className?: string }) {
  const [reducedMotion, setReducedMotion] = useState(false);

  useEffect(() => {
    const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
    setReducedMotion(mq.matches);
    const onChange = (e: MediaQueryListEvent) => setReducedMotion(e.matches);
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  return (
    <div className={cn("flex flex-col items-center justify-center gap-4", className)}>
      <div className="flex items-center gap-2.5" role="status" aria-label="Memuat">
        {[0, 1, 2].map((i) => (
          <span
            key={i}
            className={cn(
              "h-3.5 w-3.5 rounded-full bg-[#15803d] dark:bg-[#15803d]",
              !reducedMotion && "animate-bounce-dot"
            )}
            style={reducedMotion ? undefined : { animationDelay: `${i * 0.15}s` }}
          />
        ))}
      </div>
      <p className={cn("text-sm font-medium text-muted-foreground", !reducedMotion && "animate-pulse-soft")}>
        Memuat...
      </p>
    </div>
  );
}
