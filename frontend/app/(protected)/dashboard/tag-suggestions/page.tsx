import { redirect } from "next/navigation";

// FIX-22a: the real implementation moved to /studio; old dashboard URLs keep
// resolving via redirect.
export default function OldTagSuggestionsPage() {
  redirect("/studio/tag-suggestions");
}
