import { redirect } from "next/navigation";

// FIX-22a: the PascalCase-field implementation was broken against the
// lowercase API contract (blank titles, /content/undefined links); the
// working studio contents page replaces it.
export default function OldContentsPage() {
  redirect("/studio/contents");
}
