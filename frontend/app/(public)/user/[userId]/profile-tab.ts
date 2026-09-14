/* 个人主页 tab 值域与 URL 归一化（#508）：服务端 page（?tab= 初始态）与
   客户端 UserProfileClient / 测试共用——无 "use client" 指令，两侧可自由导入。 */

export type ProfileTab = "contents" | "discussions" | "collections";

export function normalizeProfileTab(raw: string | undefined): ProfileTab {
  return raw === "discussions" || raw === "collections" ? raw : "contents";
}
