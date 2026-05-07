import { cn } from "@/lib/utils";

type TagColor = "blue" | "green" | "purple" | "orange";

interface TagBadgeProps {
  className?: string;
  color?: TagColor;
  children: React.ReactNode;
  onClick?: () => void;
  onRemove?: () => void;
}

const colorStyles: Record<TagColor, string> = {
  blue: "bg-[var(--tag-blue-bg)] text-[var(--tag-blue-fg)] border-transparent",
  green: "bg-[var(--tag-green-bg)] text-[var(--tag-green-fg)] border-transparent",
  purple:
    "bg-[var(--tag-purple-bg)] text-[var(--tag-purple-fg)] border-transparent",
  orange:
    "bg-[var(--tag-orange-bg)] text-[var(--tag-orange-fg)] border-transparent",
};

export function TagBadge({
  className,
  color = "blue",
  children,
  onClick,
  onRemove,
}: TagBadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-lg border px-2 py-0.5 text-xs font-medium shadow-none",
        colorStyles[color],
        onClick && "cursor-pointer hover:opacity-80 transition-opacity",
        className,
      )}
      onClick={onClick}
      role={onClick ? "button" : undefined}
      tabIndex={onClick ? 0 : undefined}
      onKeyDown={
        onClick
          ? (e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onClick();
              }
            }
          : undefined
      }
    >
      {children}
      {onRemove && (
        <button
          type="button"
          className="ml-0.5 inline-flex items-center justify-center opacity-60 hover:opacity-100 transition-opacity"
          onClick={(e) => {
            e.stopPropagation();
            onRemove();
          }}
          aria-label="Remove tag"
        >
          <svg
            className="h-3 w-3"
            fill="currentColor"
            viewBox="0 0 12 12"
          >
            <path d="M3.47 3.47a.75.75 0 0 1 1.06 0L6 4.94l1.47-1.47a.75.75 0 1 1 1.06 1.06L7.06 6l1.47 1.47a.75.75 0 1 1-1.06 1.06L6 7.06 4.53 8.53a.75.75 0 0 1-1.06-1.06L4.94 6 3.47 4.53a.75.75 0 0 1 0-1.06Z" />
          </svg>
        </button>
      )}
    </span>
  );
}
