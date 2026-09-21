package diffengine

import (
	"fmt"
	"github.com/sergi/go-diff/diffmatchpatch"
)

var dmp = diffmatchpatch.New()

func ComputePatch(original, modified string) string {
	diffs := dmp.DiffMain(original, modified, true)
	patches := dmp.PatchMake(original, diffs)
	return dmp.PatchToText(patches)
}

func ApplyPatch(base, patchText string) (string, error) {
	patches, err := dmp.PatchFromText(patchText)
	if err != nil {
		return "", err
	}
	// SP-25 低-34：PatchApply 逐 patch 返回成功位，丢弃 []bool 会把
	// 「补丁没打上」伪装成成功——失败显性化（带版本号由调用方拼装）。
	result, oks := dmp.PatchApply(patches, base)
	for i, ok := range oks {
		if !ok {
			return "", fmt.Errorf("patch %d/%d failed to apply (base content diverged)", i+1, len(patches))
		}
	}
	return result, nil
}

func DiffSummary(original, modified string) string {
	diffs := dmp.DiffMain(original, modified, true)
	dmp.DiffCleanupSemantic(diffs)

	added, deleted := 0, 0
	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffInsert:
			added += len([]rune(d.Text))
		case diffmatchpatch.DiffDelete:
			deleted += len([]rune(d.Text))
		}
	}

	if added == 0 && deleted == 0 {
		return "no changes"
	}
	return "+" + itoa(added) + " -" + itoa(deleted) + " chars"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}
