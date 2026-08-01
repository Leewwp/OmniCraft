"use client";

import { useRouter } from "next/navigation";
import { RegisterPageContent } from "@/components/auth/RegisterPageContent";
import { useAuth } from "@/contexts/AuthContext";

export default function RegisterPage() {
  const router = useRouter();
  const { user } = useAuth();

  return <RegisterPageContent user={user} router={router} />;
}
