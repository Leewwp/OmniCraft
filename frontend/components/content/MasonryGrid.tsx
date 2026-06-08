"use client";

import { useTranslations } from "next-intl";
import { FileText } from "lucide-react";
import Masonry from "react-masonry-css";
import { ContentCard, ContentCardData } from "@/components/content/ContentCard";
import { EmptyState } from "@/components/ui/empty-state";

interface MasonryGridProps {
  items: ContentCardData[];
  emptyText?: string;
}

const breakpoints = {
  default: 4,
  1100: 3,
  700: 2,
};

export function MasonryGrid({ items, emptyText }: MasonryGridProps) {
  const t = useTranslations();

  if (items.length === 0) {
    return (
      <EmptyState
        icon={FileText}
        title={emptyText || t("content.emptyContentMsg")}
        description={t("content.emptyContentHint")}
        className="p-8"
      />
    );
  }

  return (
    <Masonry
      breakpointCols={breakpoints}
      className="-ml-4 flex w-auto"
      columnClassName="pl-4 space-y-4"
    >
      {items.map((item) => (
        <ContentCard key={item.id} data={item} />
      ))}
    </Masonry>
  );
}
