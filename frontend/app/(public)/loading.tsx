import { Skeleton, SkeletonCard } from "@/components/ui/skeleton";

export default function HomeLoading() {
  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6">
      {/* IP section skeleton */}
      <section>
        <Skeleton className="mb-3 h-5 w-24" />
        <div className="flex gap-3 overflow-hidden">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-[72px] w-48 shrink-0 rounded-md" />
          ))}
        </div>
      </section>

      {/* Content masonry skeleton */}
      <section>
        <Skeleton className="mb-3 h-5 w-32" />
        <div className="grid grid-cols-2 gap-4 md:grid-cols-3 lg:grid-cols-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <SkeletonCard key={i} />
          ))}
        </div>
      </section>
    </div>
  );
}
