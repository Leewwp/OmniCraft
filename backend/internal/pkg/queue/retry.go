package queue

var DefaultRetryBackoffSec = []int{10, 60, 300}

func RetryBackoff(cfg *QueueConfig) []int {
	if len(cfg.RetryBackoffSec) == 0 {
		return DefaultRetryBackoffSec
	}
	return cfg.RetryBackoffSec
}