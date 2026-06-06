import * as React from "react";
import { cn } from "@/lib/utils";
import { Label } from "@/components/ui/label";

function Field({ className, ...props }: React.ComponentProps<"div">) {
  return <div className={cn("flex flex-col gap-1.5", className)} {...props} />;
}

function FieldLabel({ className, ...props }: React.ComponentProps<typeof Label>) {
  return <Label className={cn("text-sm font-medium", className)} {...props} />;
}

function FieldHint({ className, ...props }: React.ComponentProps<"p">) {
  return <p className={cn("text-xs text-muted-foreground", className)} {...props} />;
}

function FieldError({ className, ...props }: React.ComponentProps<"p">) {
  return <p className={cn("text-xs text-destructive", className)} role="alert" {...props} />;
}

export { Field, FieldLabel, FieldHint, FieldError };
