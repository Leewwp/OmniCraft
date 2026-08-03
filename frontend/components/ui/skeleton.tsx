import { cn } from "@/lib/utils"

function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="skeleton"
      aria-hidden="true"
      className={cn(
        "animate-[pulse_1.6s_ease-in-out_infinite] rounded-md bg-canvas-subtle motion-reduce:animate-none",
        className,
      )}
      {...props}
    />
  )
}

function SkeletonCard({
  className,
  zone = "fanwork",
  count = 1,
  ...props
}: React.ComponentProps<"div"> & {
  zone?: "original" | "fanwork";
  count?: number;
}) {
  return Array.from({ length: count }, (_, index) => (
    <div
      key={index}
      data-slot="skeleton-card"
      data-zone={zone}
      aria-hidden="true"
      className={cn(
        "animate-[pulse_1.6s_ease-in-out_infinite] rounded-lg border border-border bg-card p-4 shadow-sm motion-reduce:animate-none",
        className,
      )}
      {...props}
    >
      <Skeleton
        className={cn(
          "mb-3 w-full rounded-lg",
          zone === "fanwork" ? "aspect-[3/4]" : "min-h-[150px]",
        )}
      />
      <Skeleton className="mb-2 h-4 w-3/4" />
      <Skeleton className="mb-2 h-3 w-1/2" />
      <div className="flex gap-2">
        <Skeleton className="h-5 w-12 rounded-full" />
        <Skeleton className="h-5 w-12 rounded-full" />
      </div>
    </div>
  ))
}

function SkeletonCircle({ size = 40, className, ...props }: React.ComponentProps<"div"> & { size?: number }) {
  return (
    <div
      data-slot="skeleton-circle"
      aria-hidden="true"
      className={cn(
        "animate-[pulse_1.6s_ease-in-out_infinite] rounded-full bg-canvas-subtle motion-reduce:animate-none",
        className,
      )}
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
      aria-hidden="true"
      className={cn(
        "animate-[pulse_1.6s_ease-in-out_infinite] space-y-4 rounded-lg border border-border bg-card p-6 shadow-sm motion-reduce:animate-none",
        className,
      )}
      {...props}
    >
      <Skeleton className="aspect-video w-full rounded-lg" />
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
