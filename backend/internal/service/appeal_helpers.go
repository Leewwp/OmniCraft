package service

func AppealTargetUpdates(targetType, appealStatus string) map[string]interface{} {
	return appealTargetUpdates(targetType, appealStatus)
}

func appealTargetUpdates(targetType, appealStatus string) map[string]interface{} {
	if targetType == "content" && appealStatus == "approved" {
		return map[string]interface{}{"status": "published"}
	}
	return map[string]interface{}{}
}
