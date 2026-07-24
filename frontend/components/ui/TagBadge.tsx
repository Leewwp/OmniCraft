"use client";

import { X } from "lucide-react";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";

type TagColor = "blue" | "green" | "purple" | "orange" | "rose" | "sky";

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
  rose:
    "bg-[var(--tag-rose-bg)] text-[var(--tag-rose-fg)] border-transparent",
  sky:
    "bg-[var(--tag-sky-bg)] text-[var(--tag-sky-fg)] border-transparent",
};

export function TagBadge({
  className,
  color = "blue",
  children,
  onClick,
  onRemove,
}: TagBadgeProps) {
  const t = useTranslations();
  const tagLabel =
    typeof children === "string" || typeof children === "number" ? String(children) : null;

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium select-none",
        colorStyles[color],
        onClick && "cursor-pointer hover:brightness-95 active:scale-95 transition-all duration-150",
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
          className="relative ml-0.5 inline-flex items-center justify-center opacity-60 transition-opacity after:absolute after:-inset-1.5 hover:opacity-100 [@media(pointer:coarse)]:after:-inset-4"
          onClick={(e) => {
            e.stopPropagation();
            onRemove();
          }}
          aria-label={
            tagLabel ? t("common.removeTag", { tag: tagLabel }) : t("common.removeTagGeneric")
          }
        >
          <X className="h-3 w-3" aria-hidden="true" />
        </button>
      )}
    </span>
  );
}
