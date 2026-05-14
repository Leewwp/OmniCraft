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
			return map[string]interface{}{"status": "published"}
		}
	}
	return map[string]interface{}{}
}
