"use client";

import Masonry from "react-masonry-css";
import { ContentCard, ContentCardData } from "@/components/content/ContentCard";

interface MasonryGridProps {
  items: ContentCardData[];
  emptyText?: string;
}

const breakpoints = {
  default: 4,
  1100: 3,
  700: 2,
  450: 1,
};

export function MasonryGrid({ items, emptyText }: MasonryGridProps) {
  if (items.length === 0) {
    return (
      <div className="rounded-md border border-border bg-card p-8 text-center text-sm text-muted-foreground">
        {emptyText || "暂无内容，稍后再来看看。"}
      </div>
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
