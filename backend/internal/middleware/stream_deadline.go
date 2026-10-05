package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// #800：http.Server.WriteTimeout 是请求起算的绝对写期限，SSE 整条流超
// 过该值必被切断（与流是否活跃无关）。StreamWriteDeadline 把挂载路由的
// 写期限改为逐写滚动延期：每次 Write/WriteString/Flush 前把期限推到
// now+window——活跃流不限总时长，静默超过 window 仍断（保留既有 idle
// 语义，去掉绝对上限）。底层链不支持 SetWriteDeadline（如测试录制器）
// 时 fail-open 维持出厂行为。
func StreamWriteDeadline(window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		rc := http.NewResponseController(c.Writer)
		if err := rc.SetWriteDeadline(time.Now().Add(window)); err != nil {
			c.Next()
			return
		}
		c.Writer = &rollingDeadlineWriter{ResponseWriter: c.Writer, rc: rc, window: window}
		c.Next()
	}
}

// rollingDeadlineWriter 在每次实际写出前滚动延期；期限设置失败不阻断
// 写出（连接若已到期限由 net/http 自身报错）。
type rollingDeadlineWriter struct {
	gin.ResponseWriter
	rc     *http.ResponseController
	window time.Duration
}

func (w *rollingDeadlineWriter) extend() {
	_ = w.rc.SetWriteDeadline(time.Now().Add(w.window))
}

func (w *rollingDeadlineWriter) Write(b []byte) (int, error) {
	w.extend()
	return w.ResponseWriter.Write(b)
}

func (w *rollingDeadlineWriter) WriteString(s string) (int, error) {
	w.extend()
	return w.ResponseWriter.WriteString(s)
}

func (w *rollingDeadlineWriter) Flush() {
	w.extend()
	w.ResponseWriter.Flush()
}
