package service

func AppealTargetUpdates(targetType, appealStatus string) map[string]interface{} {
	return appealTargetUpdates(targetType, appealStatus)
}

func appealTargetUpdates(targetType, appealStatus string) map[string]interface{} {
	if appealStatus == "rejected" {
		return map[string]interface{}{}
	}
	if appealStatus == "approved" {
		switch targetType {
		case "content":
			return map[string]interface{}{"status": "published"}
		case "comment":
			// T31（FIX-27）：评论隐藏态是 status=hidden 的软删语义，
			// 批准申诉 = 恢复展示（published⇄hidden 互逆）。
			return map[string]interface{}{"status": "published"}
		case "account":
			// T29（FIX-15）：账号申诉批准 = 解封 + 清 ban_reason。
			return map[string]interface{}{"is_banned": false, "ban_reason": ""}
		}
	}
	return map[string]interface{}{}
}
