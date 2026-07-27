// 独立スクロール領域の「器の形」を覚える（01KYH8GX987GQX08C56G58JP2N）。
//
// 位置を覚えるだけでは足りない、というのがあの decision の主旨だった。戻ってきたときに
// 領域の中身が離脱前と違う形で組み直されると、覚えていた位置がそもそも存在せず、ブラウザは
// そこへ動かせない。保存も復元も正しく動いているのに、利用者から見ると位置が失われる。
// だから位置と対で、その領域が「どういう形だったか」も覚える。
//
// 保持先と寿命は**位置と同じ**にする（同じタブのセッション限り・タブを閉じると消える・
// reload は越える）。ここを恒久側に置くと、位置は消えているのに形だけ残るという噛み合わない
// 状態になり、どちらも「そのタブでの閲覧文脈」という同じ性質のものだという整理
// （01KXF5EF61CJGTTHJH2PS93ED9 の保持先の分割）から外れる。
//
// 領域の識別子は位置側と同じものを使い（例: `overview:tree`）、接頭辞だけ分ける。位置と形が
// 対であることがキーを見て分かるようにするため。
//
// 「形」が何を指すかは領域ごとに異なる（decision がそう定めている）ので、この口は中身を
// 解釈せず JSON として預かるだけにする。
const KEY_PREFIX = 'scholia-shape-';

export function loadRegionShape<T>(region: string): T | null {
  try {
    const raw = sessionStorage.getItem(KEY_PREFIX + region);
    if (raw === null) return null;
    return JSON.parse(raw) as T;
  } catch {
    // 保存が使えない環境（private mode 等）や壊れた値。位置側と同じく「覚えていない」へ
    // 縮退させ、描画は止めない。
    return null;
  }
}

export function saveRegionShape<T>(region: string, shape: T): void {
  try {
    sessionStorage.setItem(KEY_PREFIX + region, JSON.stringify(shape));
  } catch {
    // ignore — 永続は best-effort（loadRegionShape 参照）。
  }
}
