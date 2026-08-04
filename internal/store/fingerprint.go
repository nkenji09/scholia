package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// Fingerprint は .scholia/ の「いまの状態」を、**中身を読まずに**表す短い文字列。
//
// 用途は1つ——長命プロセス（`scholia view`）が建てた in-memory index を、
// 次の要求で建て直す必要があるかどうかを決めること（語彙 cond.index-built /
// decision 01KZ5N5CJ2VFMZAGSFPSCZAMTZ 条項2「起動時に建て、`.scholia/` が
// 変わったときだけ建て直す」）。LoadAll は 4 カテゴリの JSON を全件パースするので
// 毎要求では回せないが、これは各ファイルの stat だけで済む。
//
// 材料は LoadAll が読むものと**同じ集合**にする（config.json・config.local.json・
// 4 カテゴリの *.json）。ここが LoadAll より狭いと、読んでいるのに変化を見ない
// ファイルができる。
//
// ⚠️ **何を捕まえないか**（射程・CLAUDE.md 6）:
//
//   - **サイズも mtime も変わらない書き換え。** mtime は ns まで見るので、
//     APFS/ext4 のような ns 精度の FS では同一 ns への2度書きが要る（実質起こらない）。
//     一方 mtime が秒精度しかない FS（HFS+・一部のネットワーク FS）では、
//     **同じ秒のうちに同じサイズで書き換えると見落とす。**
//   - **ファイルの中身そのものの正しさ。** 壊れた JSON かどうかは読まないと分からない。
//     これは LoadAll 側の仕事で、ここは「読み直すべきか」だけを答える。
//   - **`.scholia/reviews/`（AI コメントのサイドカー）。** LoadAll の対象外なので
//     ここでも見ない（reviews は独自に毎回読む）。
//
// stat に失敗したら err を返す。呼び出し側は「分からない＝建て直す」に倒すこと。
func (s *Store) Fingerprint() (string, error) {
	h := sha256.New()

	for _, name := range []string{configFile, localConfigFile} {
		info, err := os.Stat(filepath.Join(s.Dir, name))
		if err != nil {
			if os.IsNotExist(err) {
				// config.local.json は「無いのが普通」。無いことも状態の一部
				// なので、消えた／現れたが指紋に出るように印を書く。
				fmt.Fprintf(h, "%s\x00-\n", name)
				continue
			}
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%d\x00%d\n", name, info.Size(), info.ModTime().UnixNano())
	}

	for _, sub := range []string{vocabDir, tagsDir, transitionsDir, decisionsDir} {
		// os.ReadDir は名前順に返すので、同じ内容なら同じ指紋になる。
		entries, err := os.ReadDir(filepath.Join(s.Dir, sub))
		if err != nil {
			if os.IsNotExist(err) {
				// LoadAll も「ディレクトリ無し＝0件」として扱う（listRecords）。
				fmt.Fprintf(h, "%s\x00-\n", sub)
				continue
			}
			return "", err
		}
		for _, e := range entries {
			// listRecords が読むものと**同じ絞り**を通す（isRecordFile）。
			// ここが広いと、書き込み中の一時ファイル（`.tmp-*.json`）が現れたり
			// 消えたりするたびに指紋が変わり、読んでいないものの変化で
			// 建て直すことになる。
			if e.IsDir() || !isRecordFile(e.Name()) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				return "", err
			}
			fmt.Fprintf(h, "%s/%s\x00%d\x00%d\n", sub, e.Name(), info.Size(), info.ModTime().UnixNano())
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
