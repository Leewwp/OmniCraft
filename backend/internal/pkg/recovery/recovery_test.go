package recovery

import (
	"testing"
	"time"
)

// #745：后台 goroutine 的 panic 防护是宪法 XV 硬约束——GoSafe 内 panic
// 必须被吞掉并留日志，绝不允许击穿进程；正常路径照常执行。
func TestGoSafeRecoversPanicWithoutCrashing(t *testing.T) {
	ran := make(chan struct{})
	GoSafe(func() {
		defer close(ran)
		panic("boom")
	})
	select {
	case <-ran:
	case <-time.After(2 * time.Second):
		t.Fatal("GoSafe did not run fn")
	}
}

func TestGoSafeRunsNormalFn(t *testing.T) {
	called := make(chan struct{})
	GoSafe(func() { close(called) })
	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("GoSafe did not run fn")
	}
}
