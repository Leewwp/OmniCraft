import { redirect } from "next/navigation";

// 旧类目子路由已并入 /ip/[ipId] 就地切换（SP-12 U-03）：
// redirect 至 query 形式，保留 sort 参数；page 参数不再适用（就地列表无分页路由）。
export default async function IPCategoryRedirectPage({
  params,
  searchParams,
}: {
  params: Promise<{ ipId: string; category: string }>;
  searchParams: Promise<{ sort?: string }>;
}) {
  const { ipId, category } = await params;
  const query = await searchParams;
  const sort = query.sort || "hot";

  const qs = new URLSearchParams();
  qs.set("category", category);
  if (sort !== "hot") qs.set("sort", sort);
  redirect(`/ip/${ipId}?${qs.toString()}`);
}
