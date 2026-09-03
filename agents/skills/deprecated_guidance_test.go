package skills

import (
	"io/fs"
	"strings"
	"testing"
)

// 配布スキルが「非推奨の口」を、非推奨と分からない形で勧めていないこと
// （01M1K4WPN3HXNVE0CR0T6NQK33）。
//
// # なぜ要るのか
//
// `commits[]` を非推奨にした判断は、**配布スキルの記述でしか読者に届かない。**
// この repo は「明文化しても遡及機構が無ければ挙動は変わらない」を実測付きで決めており
// （01KXS68HCNQ0H9QKNYFQ869J19）、記述だけの非推奨はまさにその型に当たる。
// 実際、非推奨にした直後の走査で**推奨の記述が 13 箇所残っていた**——書き換えたつもりで
// 残る、が既定の結果である。
//
// # 落ちる範囲（CLAUDE.md「配線ガードの書き方」6）
//
// **落ちる:**
//   - 非推奨の口を勧める行が、非推奨の印を持たずに新しく書かれたとき。
//
// **落ちない（射程の外・正直に名乗る）:**
//   - 🔴 **これはソース文字列の照合である**（CLAUDE.md 2）。同じ意味を別の綴りで
//     書けば通る——「commit を結びます」と散文で書く、コマンドを分割して書く、等。
//     **捕まえられない綴りを1つずつ列挙してはいけない**ので、ここは列挙しない。
//     見ているのは「そのコマンド文字列が印なしで現れるか」ひとつだけである。
//   - **印が正しいかどうか。** 同じ行に「非推奨」と書いてあれば通るので、
//     文脈が逆（「非推奨ではない」）でも通る。実際 `--kind correction` の行は
//     **意図的に**その形で通している。
//   - **配布物以外**（DESIGN.md・実装のヘルプ文字列）。ここが見るのは embed される
//     スキル文書だけである。
func TestSkillsDoNotRecommendDeprecatedCommitLinking(t *testing.T) {
	// 非推奨にした口。**`--kind correction` は含めない**——是正の印を打つ唯一の口で、
	// 非推奨ではない（01M1K4WPN3HXNVE0CR0T6NQK33）。
	deprecated := []string{
		"add-commit <decisionId> <hash> --kind implementation",
		"add-commit <id> <hash> --kind implementation",
		"decide --commit",
		"--commit <hash>",
	}
	// 同じ行にこれがあれば「非推奨と分かる形」とみなす。
	marks := []string{"非推奨", "deprecated"}

	var offenders []string
	err := fs.WalkDir(FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		b, err := FS.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			for _, dep := range deprecated {
				if !strings.Contains(line, dep) {
					continue
				}
				marked := false
				for _, m := range marks {
					if strings.Contains(line, m) {
						marked = true
						break
					}
				}
				if !marked {
					offenders = append(offenders, path+":"+itoa(i+1)+": "+strings.TrimSpace(line))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("スキル文書を走査できません: %v", err)
	}
	if len(offenders) > 0 {
		t.Errorf(`配布スキルが非推奨の口を、非推奨と分からない形で勧めている:

%s

来歴を結ぶ正本は refs[]（scholia decision add-ref / decide --refs）である
——commit hash は取り込み（squash merge・作業ブランチの作り直し）で辿れなくなる
（01M1K4WPN3HXNVE0CR0T6NQK33）。書き換えるか、同じ行に非推奨であることを書くこと。
⚠️ --kind correction は非推奨ではない（是正の印を打つ唯一の口）。`, strings.Join(offenders, "\n"))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
