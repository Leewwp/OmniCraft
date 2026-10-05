// Package contentroute（#793）：内容详情路由 = f(zone, id) 的唯一后端真源。
//
// 前端镜像为 frontend/lib/content.ts 的 getContentHref（#781 收敛），两侧规则必须
// 一致：zone == "original" → /original/{id}（/content/{id} 对 zone=original 有意
// notFound，两详情路由分区隔离为设计行为）；fanwork / 缺省 / 未知 zone 一律回退
// /content/{id}。
//
// 纯函数 leaf module：无 SQL、无 IO、无状态。contentID 的合法性校验仍由 caller
// 持有（本函数不改任何错误合同，也不会把非法 ID 变成合法路由）；IP 引用路由
// （/ip/{id}）独立于内容分区语义，不属本 module。
package contentroute

import "strconv"

// ContentDetailRoute 返回内容详情深链：
//   - zone == "original"       → /original/{contentID}
//   - fanwork / 空 / 未知 zone → /content/{contentID}（历史回退，行为不变）
func ContentDetailRoute(zone string, contentID int64) string {
	if zone == "original" {
		return "/original/" + strconv.FormatInt(contentID, 10)
	}
	return "/content/" + strconv.FormatInt(contentID, 10)
}
