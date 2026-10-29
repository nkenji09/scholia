package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// useQuietReader は、候補を返さない偽の読み手を環境変数で指定する。
// 読み手の指定が無いと失敗（終了コード 1）する text-check を、面を一巡する検査から走らせるために使う。
func useQuietReader(t *testing.T) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "reader")
	if err := os.WriteFile(p, []byte("#!/bin/sh\ncat > /dev/null\necho '{\"candidates\":[]}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SCHOLIA_TEXT_READER", p)
}
