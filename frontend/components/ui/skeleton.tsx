import { cn } from "@/lib/utils"

function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="skeleton"
      className={cn("animate-pulse rounded-md bg-muted", className)}
      {...props}
    />
  )
}

function SkeletonCard({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="skeleton-card"
      className={cn("animate-pulse rounded-md border border-border bg-card p-4", className)}
      {...props}
    >
      <Skeleton className="mb-3 aspect-[3/4] w-full rounded-md" />
      <Skeleton className="mb-2 h-4 w-3/4" />
      <Skeleton className="mb-2 h-3 w-1/2" />
      <div className="flex gap-2">
        <Skeleton className="h-5 w-12 rounded-full" />
        <Skeleton className="h-5 w-12 rounded-full" />
      </div>
    </div>
  )
}

function SkeletonCircle({ size = 40, className, ...props }: React.ComponentProps<"div"> & { size?: number }) {
  return (
    <div
      data-slot="skeleton-circle"
      className={cn("animate-pulse rounded-full bg-muted", className)}
      style={{ width: size, height: size }}
      {...props}
    />
  )
}

function SkeletonText({
  lines = 3,
  className,
  ...props
}: React.ComponentProps<"div"> & { lines?: number }) {
  return (
    <div data-slot="skeleton-text" className={cn("space-y-2", className)} {...props}>
      {Array.from({ length: lines }).map((_, i) => (
        <Skeleton
          key={i}
          className={cn("h-3", i === lines - 1 ? "w-2/3" : "w-full")}
        />
      ))}
    </div>
  )
}

function SkeletonDetail({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="skeleton-detail"
      className={cn("animate-pulse space-y-4 rounded-md border border-border bg-card p-6", className)}
      {...props}
    >
      <Skeleton className="aspect-video w-full rounded-md" />
      <Skeleton className="h-6 w-2/3" />
      <Skeleton className="h-4 w-1/3" />
      <SkeletonText lines={4} />
      <div className="flex gap-3">
        <Skeleton className="h-9 w-20 rounded-md" />
        <Skeleton className="h-9 w-20 rounded-md" />
        <Skeleton className="h-9 w-20 rounded-md" />
      </div>
    </div>
  )
}

export { Skeleton, SkeletonCard, SkeletonCircle, SkeletonText, SkeletonDetail }
