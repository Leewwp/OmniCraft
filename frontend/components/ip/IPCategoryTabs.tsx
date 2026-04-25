"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import { ipCategoryOptions } from "@/components/ip/ipCategory";

interface IPCategoryTabsProps {
  ipId: string;
  activeCategory?: string;
}

export function IPCategoryTabs({ ipId, activeCategory = "all" }: IPCategoryTabsProps) {
  const searchParams = useSearchParams();
  const sort = searchParams.get("sort") || "hot";

  return (
    <div className="flex flex-wrap gap-2">
      {ipCategoryOptions.map((item) => {
        const active = item.key === activeCategory;
        const href =
          item.key === "all"
            ? `/ip/${ipId}?sort=${sort}`
            : `/ip/${ipId}/${item.key}?sort=${sort}`;

        return (
          <Link key={item.key} href={href}>
            <Button size="sm" variant={active ? "default" : "outline"}>
              {item.label}
            </Button>
          </Link>
        );
      })}
    </div>
  );
}
