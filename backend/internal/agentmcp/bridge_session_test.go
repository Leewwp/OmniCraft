package agentmcp

import (
	"context"
	"sync"
	"testing"
	"time"

	"omnicraft/backend/config"
)

// SP-25 低-10：会话建立（握手 + ListTools，慢 server 最长 ~30s）必须在互斥锁
// 外执行——一个慢/死 server 不得阻塞其他 server 的会话建立；同 server 的并发
// 调用经 singleflight 共享一次连接。

func stubbedBridge(connect func(ctx context.Context, srv config.AgentMCPServerConfig) (*serverSession, error)) *Bridge {
	b := New(config.AgentMCPConfig{Enabled: true, CallTimeoutSec: 5, ResultMaxBytes: 4096})
	b.connectFn = connect
	return b
}

func TestSessionSlowServerDoesNotBlockOthers(t *testing.T) {
	releaseSlow := make(chan struct{})
	b := stubbedBridge(func(ctx context.Context, srv config.AgentMCPServerConfig) (*serverSession, error) {
		if srv.ID == "slow" {
			<-releaseSlow // 模拟握手挂起，直到测试放行
		}
		return &serverSession{localOf: map[string]string{}}, nil
	})

	go func() { _, _ = b.session(context.Background(), config.AgentMCPServerConfig{ID: "slow"}) }()

	// slow 尚在连接中：fast server 必须能立即完成建立，不被全局锁卡住。
	done := make(chan struct{})
	go func() {
		defer close(done)
		sess, err := b.session(context.Background(), config.AgentMCPServerConfig{ID: "fast"})
		if err != nil {
			t.Errorf("fast session: %v", err)
		}
		if sess == nil {
			t.Error("fast session must not be nil")
		}
	}()
	select {
	case <-done:
		// pass
	case <-time.After(3 * time.Second):
		t.Fatal("fast server session blocked behind a slow server's connect (session establishment must run outside the mutex)")
	}
	close(releaseSlow)
}

func TestSessionConcurrentSameServerSharesOneConnect(t *testing.T) {
	var connects sync.WaitGroup
	connects.Add(1)
	started := make(chan struct{})
	b := stubbedBridge(func(ctx context.Context, srv config.AgentMCPServerConfig) (*serverSession, error) {
		close(started)
		time.Sleep(100 * time.Millisecond)
		connects.Done()
		return &serverSession{localOf: map[string]string{}}, nil
	})

	const callers = 5
	var wg sync.WaitGroup
	results := make([]*serverSession, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sess, err := b.session(context.Background(), config.AgentMCPServerConfig{ID: "doc"})
			if err != nil {
				t.Errorf("caller %d: %v", i, err)
			}
			results[i] = sess
		}(i)
	}
	wg.Wait()

	<-started
	// 所有调用者必须拿到同一个会话实例（单飞共享，无重复子进程）。
	for i := 1; i < callers; i++ {
		if results[i] != results[0] {
			t.Fatalf("caller %d got a different session instance; concurrent connects for one server must be deduped", i)
		}
	}
}
