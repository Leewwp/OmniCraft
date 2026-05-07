import { Skeleton, SkeletonCard } from "@/components/ui/skeleton";

export default function IPDetailLoading() {
  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6">
      {/* IP info skeleton */}
      <div className="rounded-md border border-border bg-card p-6">
        <div className="flex items-start gap-4">
          <Skeleton className="h-20 w-20 shrink-0 rounded-md" />
          <div className="flex-1 space-y-2">
            <Skeleton className="h-6 w-48" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-3/4" />
          </div>
        </div>
      </div>

      {/* Content grid skeleton */}
      <section className="rounded-md border border-border bg-card p-4">
        <Skeleton className="mb-3 h-5 w-24" />
        <div className="grid grid-cols-2 gap-4 md:grid-cols-3 lg:grid-cols-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <SkeletonCard key={i} />
          ))}
        </div>
      </section>
    </div>
  );
}
