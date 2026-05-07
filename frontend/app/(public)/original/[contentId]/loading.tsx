import { Skeleton, SkeletonText } from "@/components/ui/skeleton";

export default function OriginalDetailLoading() {
  return (
    <div className="mx-auto flex w-full max-w-4xl flex-col gap-6 px-4 py-6">
      <div className="rounded-md border border-border bg-card p-6 space-y-4">
        <Skeleton className="aspect-video w-full rounded-md" />
        <Skeleton className="h-7 w-2/3" />
        <div className="flex gap-2">
          <Skeleton className="h-5 w-16 rounded-full" />
          <Skeleton className="h-5 w-16 rounded-full" />
        </div>
        <SkeletonText lines={5} />
        <div className="flex flex-wrap gap-3">
          <Skeleton className="h-9 w-24 rounded-md" />
          <Skeleton className="h-9 w-24 rounded-md" />
          <Skeleton className="h-9 w-32 rounded-md" />
        </div>
      </div>
    </div>
  );
}
