package viewer

import (
	"encoding/json"
	"net/http"
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
