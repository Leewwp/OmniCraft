import { redirect } from "next/navigation";

export default function OldDashboardPage() {
  redirect("/studio/overview");
}
