export function Toggle({
  checked,
  onToggle,
  disabled,
  label,
  title,
}: {
  checked: boolean;
  onToggle: (v: boolean) => void;
  disabled?: boolean;
  label: string;
  title?: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      title={title}
      disabled={disabled}
      onClick={() => onToggle(!checked)}
      className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border transition-colors disabled:opacity-50 disabled:cursor-not-allowed ${
        checked
          ? "bg-[#15803d] border-[#15803d] dark:bg-[#15803d] dark:border-[#15803d]"
          : "bg-muted border-border dark:bg-muted dark:border-muted"
      }`}
    >
      <span
        className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform ${
          checked ? "translate-x-4" : "translate-x-0.5"
        }`}
      />
    </button>
  );
}
