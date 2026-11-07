import PrototypeLanding from "@/components/home/prototype-landing";
import type { Metadata } from "next";
import { canonicalAlternates } from "@/lib/seo";

// 票 #950：仅补 canonical（title 等维持根 layout 回退，无可见变化）。
export const metadata: Metadata = { alternates: canonicalAlternates("/home") };

// Landing page — non-logged-in visitors see brand intro.
// Logged-in users are redirected to / (fanwork zone).
export default function HomePage() {
  return <PrototypeLanding />;
}
