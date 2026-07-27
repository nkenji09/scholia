package viewer

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/nkenji09/scholia/internal/review"
	"github.com/nkenji09/scholia/internal/store"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

type errorBody struct {
	Error string `json:"error"`
	// Code は機械可読な失敗種別（omitempty）。フロントはこれを見て自前の
	// （翻訳済み・生 id を含まない）文言を選ぶ——viewer は生レコード id を
	// 表示しない（01KYCC2TF3NW3JRSSRK9ZHN078）ので、サーバの文言をそのまま
	// 出すだけでは id が漏れる面がある。Code を知らないフロントは従来どおり
	// Error をそのまま出せばよい。
	Code string `json:"code,omitempty"`
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorBody{Error: msg})
}

func writeErrorCode(w http.ResponseWriter, status int, msg, code string) {
	writeJSON(w, status, errorBody{Error: msg, Code: code})
}

// writeStoreError は「レコードファイルが1件読めない／壊れている」失敗を、生の
// ファイル名（decision と review は `<ULID>.json`）を出さずに返す。
//
// ここが難しいのは、01KYCC2TF3NW3JRSSRK9ZHN078（viewer は生レコード id を表示
// しない）と「壊れたレコードに到達できる」が正面から当たるため。ファイル名を
// 落とすだけだと、直す対象が分からなくなって修復不能になる。両方を立てるために
// 役割を分ける:
//
//   - **どのレコード種別が・どう壊れているか**は viewer が読ませる（decisions か
//     tags か、JSON 構文エラーか読み取り失敗か）。ここまでで対象は1ディレクトリに
//     絞れる。
//   - **どのファイルか**は端末が担う。`scholia lint` など store を読む CLI は
//     どれも同じエラーでファイル名を出す（CLI は id を出してよい面）。viewer は
//     そのコマンドを案内する。
//
// 壊れたファイルを直す作業自体が端末での作業なので、到達の最後の一歩を CLI に
// 渡すのは動線として自然でもある。id を読ませずに直せる、が成立している。
func writeStoreError(w http.ResponseWriter, err error) {
	var recErr *store.RecordFileError
	if errors.As(err, &recErr) {
		// 4 つのレコードディレクトリは store.LoadAll が読むので、lint が同じ
		// エラーでファイル名を出す。
		writeErrorCode(w, http.StatusInternalServerError,
			unreadableRecordMessage(recordDirLabel(recErr.Category), "scholia lint", recErr.Parse),
			"record-file-unreadable")
		return
	}
	var revErr *review.FileError
	if errors.As(err, &revErr) {
		// 提案コメントは LoadAll の対象外（§8.4: reviews/ は lint から見えない）
		// なので lint では出ない。名前を出すのは review を読む経路。
		writeErrorCode(w, http.StatusInternalServerError,
			unreadableRecordMessage(".scholia/reviews/", "scholia review list", revErr.Parse),
			"record-file-unreadable")
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

// unreadableRecordMessage は「どのディレクトリが・どう壊れているか」と、
// 「ファイル名を出すコマンド」を伝える。cmd を引数にするのは、reviews/ が
// store.LoadAll の対象外で `scholia lint` では検出されないため——案内を1つに
// 固定すると、提案コメントが壊れたときに存在しない導線を指してしまう。
func unreadableRecordMessage(dir, cmd string, parse bool) string {
	how := "読み取りに失敗しました"
	if parse {
		how = "JSON として壊れています"
	}
	return fmt.Sprintf("%s のレコードファイルを1件読み込めません（%s）。端末で `%s` を実行すると、対象のファイル名と原因が表示されます。", dir, how, cmd)
}

// recordDirLabel は store のカテゴリ名を .scholia 配下のディレクトリ表記へ直す
// （利用者が実際に開く場所で示す）。
func recordDirLabel(category string) string {
	switch category {
	case "vocab":
		return ".scholia/vocab/"
	case "tag":
		return ".scholia/tags/"
	case "transition":
		return ".scholia/transitions/"
	case "decision":
		return ".scholia/decisions/"
	}
	return ".scholia/"
}
