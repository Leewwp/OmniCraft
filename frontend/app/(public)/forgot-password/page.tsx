"use client";

import { Suspense } from "react";
import { ForgotPasswordContent } from "@/components/auth/ForgotPasswordContent";

export default function ForgotPasswordPage() {
  return (
    <Suspense
      fallback={<div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center" />}
    >
      <ForgotPasswordContent />
    </Suspense>
  );
}
