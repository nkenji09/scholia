package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const defaultEditor = "vi"

// runEditor は $EDITOR を実際に起動する処理の seam。テストでは EDITOR=cat 等の
// 非対話コマンドに差し替えるだけで済むよう、この変数自体は差し替えない
// （$EDITOR の値を尊重する方式でテスト可能にする）。
var runEditor = func(editorCmd string, path string) error {
	fields := strings.Fields(editorCmd)
	if len(fields) == 0 {
		return fmt.Errorf("$EDITOR が空です")
	}
	cmd := exec.Command(fields[0], append(fields[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// captureFromEditor は $EDITOR（未設定なら vi）で一時ファイルを開き、保存された
// 内容を説明本文として返す。末尾改行のみ trim し、中身は markdown のまま保持する。
func captureFromEditor() (string, error) {
	editorCmd := os.Getenv("EDITOR")
	if editorCmd == "" {
		editorCmd = defaultEditor
	}

	f, err := os.CreateTemp("", "scholia-desc-*.md")
	if err != nil {
		return "", fmt.Errorf("一時ファイルを作成できません: %w", err)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	if err := runEditor(editorCmd, path); err != nil {
		return "", fmt.Errorf("$EDITOR (%q) の起動に失敗しました: %w", editorCmd, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("編集内容を読み込めません: %w", err)
	}
	return strings.TrimRight(string(data), "\n"), nil
}

// readDescFile はファイルから説明本文を読み込む。末尾改行のみ trim し、
// 複数段落の markdown はそのまま保持する。
func readDescFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("--desc-file %q を読み込めません: %w", path, err)
	}
	return strings.TrimRight(string(data), "\n"), nil
}

// descSource は説明入力の3経路（直接文字列・ファイル・$EDITOR）の排他制御と
// 値解決をまとめる。directSet は該当フラグが cmd.Flags().Changed(...) かどうか。
type descSource struct {
	direct    string
	directSet bool
	file      string
	edit      bool
}

// resolve は説明本文を解決する。changed=false は「何も指定されなかった」ことを示し、
// 呼び出し側は create では既定値（空文字）、edit 系では既存値維持に使う。
func (d descSource) resolve() (value string, changed bool, err error) {
	set := 0
	if d.directSet {
		set++
	}
	if d.file != "" {
		set++
	}
	if d.edit {
		set++
	}
	if set > 1 {
		return "", false, fmt.Errorf("--desc/--description・--desc-file・--edit は同時に指定できません（いずれか1つを指定してください）")
	}

	switch {
	case d.file != "":
		s, err := readDescFile(d.file)
		return s, true, err
	case d.edit:
		s, err := captureFromEditor()
		return s, true, err
	case d.directSet:
		return d.direct, true, nil
	default:
		return "", false, nil
	}
}
