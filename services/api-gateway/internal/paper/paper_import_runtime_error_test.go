package paper

import "testing"

func TestPaperImportRuntimeErrorMessageExplainsSourceDownloadFailures(t *testing.T) {
	tests := map[string]string{
		"source_download_forbidden": "源文件访问被拒绝，请重新提交资料或联系管理员",
		"source_not_found":          "源文件不存在或已删除",
		"source_download_failed":    "源文件读取失败，请稍后重试",
	}
	for code, want := range tests {
		if got := paperImportRuntimeErrorMessage(code); got != want {
			t.Fatalf("%s: got %q, want %q", code, got, want)
		}
	}
}
