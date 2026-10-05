package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// #806 A1 守门（source-contract，先例 pagination_gate_test.go）：handler
// 层路径参数的数字解析只许走 path_param.go 的 pathID——非测试源码中出现
// 裸 ParseInt(c.Param(...)) / Atoi(c.Param(...)) 即失败。
// 白名单 = 登记例外（独立契约）：
//   - path_param.go：pathID 契约定义处（解析本体）；
//   - series.go：parseSeriesParam 走 VALIDATION_ERROR 族（独立信封契约）；
//   - ip_visit_history.go：坏 ipId 映射 404 IP_NOT_FOUND（反枚举契约，
//     刻意不暴露 400/404 区分）。

var rawPathParamParsePattern = regexp.MustCompile(`(ParseInt|Atoi)\(c\.Param\(`)

var pathParamParseWhitelist = map[string]string{
	"path_param.go":       "pathID 契约定义处（解析本体）",
	"series.go":           "parseSeriesParam VALIDATION_ERROR 族契约",
	"ip_visit_history.go": "404 IP_NOT_FOUND 反枚举契约",
}

func TestHandlersHaveNoRawPathParamParse(t *testing.T) {
	for _, file := range handlerSourceFiles(t) {
		if _, whitelisted := pathParamParseWhitelist[file]; whitelisted {
			// 登记例外：保留独立契约（理由见上方注释与本表）。
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if rawPathParamParsePattern.Match(raw) {
			lines := []string{}
			for i, line := range strings.Split(string(raw), "\n") {
				if rawPathParamParsePattern.MatchString(line) {
					lines = append(lines, filepath.Base(file)+":"+itoa(i+1))
				}
			}
			t.Fatalf("%s 出现裸路径参数解析（须走 pathID）：\n%s\n若确属独立契约，先在 path_param.go 注释与本测试白名单登记理由",
				file, strings.Join(lines, "\n"))
		}
	}
}

func TestPathParamGateCatchesViolations(t *testing.T) {
	// 反例：新增裸解析必须被守门拦下（谓词有效性证明）。
	violation := []byte(`func getX(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	_ = id
	_ = err
}`)
	if !rawPathParamParsePattern.Match(violation) {
		t.Fatal("gate pattern must match a new bare ParseInt(c.Param(...)) parse")
	}
	violationAtoi := []byte(`id, err := strconv.Atoi(c.Param("id"))`)
	if !rawPathParamParsePattern.Match(violationAtoi) {
		t.Fatal("gate pattern must match a bare Atoi(c.Param(...)) parse")
	}
	clean := []byte(`id, ok := pathID(c, "id", "invalid x id")`)
	if rawPathParamParsePattern.Match(clean) {
		t.Fatal("gate pattern must not match pathID usage")
	}
	// 白名单例外文件模式本身可被识别。
	if _, ok := pathParamParseWhitelist["series.go"]; !ok {
		t.Fatal("declared exception series.go must be in the whitelist")
	}
}

func TestPathIDContract(t *testing.T) {
	cases := []struct {
		name     string
		param    string
		message  string
		wantID   int64
		wantOK   bool
		wantCode int
		wantBody string
	}{
		{name: "valid id", param: "42", message: "invalid x id", wantID: 42, wantOK: true},
		{name: "garbage", param: "abc", message: "invalid x id", wantCode: http.StatusBadRequest, wantBody: `{"code":"INVALID_ID","message":"invalid x id"}`},
		{name: "zero rejected", param: "0", message: "invalid x id", wantCode: http.StatusBadRequest, wantBody: `{"code":"INVALID_ID","message":"invalid x id"}`},
		{name: "negative rejected", param: "-5", message: "invalid x id", wantCode: http.StatusBadRequest, wantBody: `{"code":"INVALID_ID","message":"invalid x id"}`},
		{name: "overflow rejected", param: "99999999999999999999", message: "invalid x id", wantCode: http.StatusBadRequest, wantBody: `{"code":"INVALID_ID","message":"invalid x id"}`},
		{name: "empty message keeps code-only envelope", param: "x", message: "", wantCode: http.StatusBadRequest, wantBody: `{"code":"INVALID_ID"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: tc.param}}
			id, ok := pathID(c, "id", tc.message)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if id != tc.wantID {
				t.Fatalf("id = %d, want %d", id, tc.wantID)
			}
			if !tc.wantOK {
				if w.Code != tc.wantCode {
					t.Fatalf("status = %d, want %d", w.Code, tc.wantCode)
				}
				if got := strings.TrimSpace(w.Body.String()); got != tc.wantBody {
					t.Fatalf("body = %s, want %s", got, tc.wantBody)
				}
			}
		})
	}
}
