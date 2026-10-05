package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// #800 先红后绿锚点：真实 http.Server 绝对 WriteTimeout 下，SSE 逐写
// 滚动延期必须让流活过绝对期限写完全部 tick；无中间件基线必须被切断。
func TestStreamWriteDeadline_RollingSurvivesAbsoluteWriteTimeout(t *testing.T) {
	const (
		writeTimeout = 150 * time.Millisecond
		writeEvery   = 50 * time.Millisecond
		totalTicks   = 8 // ~400ms，远超 150ms 绝对期限
	)
	handler := func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		for i := 0; i < totalTicks; i++ {
			fmt.Fprintf(c.Writer, "data: tick %d\n\n", i)
			c.Writer.Flush()
			time.Sleep(writeEvery)
		}
	}
	boot := func(withMiddleware bool) *httptest.Server {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		if withMiddleware {
			r.GET("/s", StreamWriteDeadline(writeTimeout), handler)
		} else {
			r.GET("/s", handler)
		}
		srv := httptest.NewUnstartedServer(r)
		srv.Config.WriteTimeout = writeTimeout
		srv.Start()
		return srv
	}
	readTicks := func(t *testing.T, url string) (ticks int, err error) {
		resp, rerr := http.Get(url)
		if rerr != nil {
			return 0, rerr
		}
		defer resp.Body.Close()
		body, rerr := io.ReadAll(resp.Body)
		if rerr != nil {
			return 0, rerr
		}
		return strings.Count(string(body), "data: tick"), nil
	}

	t.Run("baseline without middleware is cut by absolute write timeout", func(t *testing.T) {
		srv := boot(false)
		defer srv.Close()
		ticks, err := readTicks(t, srv.URL+"/s")
		require.True(t, err != nil || ticks < totalTicks,
			"absolute write_timeout must truncate the stream: ticks=%d err=%v", ticks, err)
	})

	t.Run("rolling deadline writes all ticks past the absolute timeout", func(t *testing.T) {
		srv := boot(true)
		defer srv.Close()
		ticks, err := readTicks(t, srv.URL+"/s")
		require.NoError(t, err)
		require.Equal(t, totalTicks, ticks, "stream must survive past the absolute write timeout")
	})
}

// deadlineRecorder 模拟支持写期限的真实连接层，记录每次 SetWriteDeadline。
type deadlineRecorder struct {
	http.ResponseWriter
	mu        sync.Mutex
	deadlines []time.Time
}

func (d *deadlineRecorder) SetWriteDeadline(t time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.deadlines = append(d.deadlines, t)
	return nil
}

func (d *deadlineRecorder) snapshot() []time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]time.Time(nil), d.deadlines...)
}

// 初始延期 + Write/WriteString/Flush 各自滚动一次。
func TestStreamWriteDeadline_ExtendsOnEveryWritePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := &deadlineRecorder{ResponseWriter: httptest.NewRecorder()}
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/s", nil)
	before := time.Now()
	StreamWriteDeadline(time.Second)(c)
	// 模拟下游 handler 的三条写出路径（此时 c.Writer 已被换成滚动包装）。
	c.Writer.Write([]byte("a"))
	c.Writer.WriteString("b")
	c.Writer.Flush()
	deadlines := rec.snapshot()
	require.GreaterOrEqual(t, len(deadlines), 4, "initial + write + writeString + flush must each extend")
	for _, d := range deadlines {
		require.True(t, d.After(before), "extensions must roll forward from now")
	}
}

// 底层链不支持 SetWriteDeadline（纯录制器）时 fail-open：请求照常处理。
func TestStreamWriteDeadline_FailOpenWithoutDeadlineSupport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/s", nil)
	StreamWriteDeadline(time.Second)(c)
	c.String(http.StatusOK, "ok")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "ok")
}
