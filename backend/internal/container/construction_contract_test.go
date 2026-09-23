package container

// #658 source-contract 守门测试：三个高风险服务构造器（审核 / 内容 /
// OSS presign）的生产构造点只允许出现在组合根（本包 NewContainer）；
// internal/testutil 是测试组合 seam，豁免。手法同 internal/router
// routes_test.go 的「路由禁构造」断言（扫描源码形态）：本批收口后，
// 下一个 PR 无法在 handler / worker / service 里悄悄重建第二套依赖图
// 而不被 CI 拦下。_test.go 一律豁免（行为测试自行组装被测面）。

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 限定名调用（service.NewXxx）：全仓生产源码禁用（组合根与 seam 外）。
var constructionContractQualified = []string{
	"service.NewReviewService(",
	"service.NewContentService(",
	"service.NewContentServiceWithOSS(",
	"service.NewOSSService(",
}

// 未限定调用（NewXxx）：service 包自身的生产文件同样不得互建
// （跳过 func 声明行）。
var constructionContractBare = []string{
	"NewReviewService(",
	"NewContentService(",
	"NewContentServiceWithOSS(",
	"NewOSSService(",
}

func TestHighRiskServiceConstructorsLiveOnlyInCompositionRoot(t *testing.T) {
	root := filepath.Join("..", "..") // backend/
	var violations []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			switch rel {
			case filepath.Join("internal", "container"),
				filepath.Join("internal", "testutil"), // 测试组合 seam 豁免
				"vendor", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, marker := range constructionContractQualified {
			if strings.Contains(string(data), marker) {
				violations = append(violations, rel+" constructs "+marker)
			}
		}
		// 未限定形态只在 service 包内检查（其它包裸名无包前缀不可达）。
		if strings.HasPrefix(rel, filepath.Join("internal", "service")) {
			for _, line := range strings.Split(string(data), "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "func ") {
					continue
				}
				for _, marker := range constructionContractBare {
					if strings.Contains(line, marker) {
						violations = append(violations, rel+" bare-constructs "+marker)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan production sources: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("high-risk service constructions must live only in container.NewContainer (testutil seam exempt):\n%s",
			strings.Join(violations, "\n"))
	}
}
