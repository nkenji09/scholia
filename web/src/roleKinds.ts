import type { KindDecl } from './types';
import { kindDeclObject } from './types';

// 概要ビュー（仕様シート）が依存する4つの「役割」。
//
// 01KYCC2THS5RX3HB27SQGFWSA5（現行）が定めたとおり、役割は**リテラル kind id 固定
// ではなく config.tagKinds の behaviors 宣言で解決する**——component 概念を別 id で
// 表すプロジェクトでも仕様シートが出るようにするため。
//
// この判定を画面から切り離してここに置いてある。「宣言をどう読むか」「宣言が無い
// ときどう落とすか」は入力（tagKinds）に対する答えとして検査できる形にしておく
// （CLAUDE.md「配線ガードの書き方」1）。
export type SheetRole = 'component' | 'part' | 'constraint' | 'group';

/** 役割 → その役割を宣言する behaviors マーカー（KindDeclObject.behaviors に含むと、
    その kind がこの役割を担う）。axis の behaviors:["axis"] と同じ仕組み。 */
export const ROLE_BEHAVIOR: Record<SheetRole, string> = {
  component: 'component',
  part: 'part',
  constraint: 'constraint',
  group: 'group',
};

/** 役割 → behaviors 宣言が無いときのリテラル kind id フォールバック。behaviors を
    宣言しない既存プロジェクト（component/part/group/property を直に使う）が従来
    どおり動く。constraint だけは歴史的経緯で property へ落ちる点に注意。 */
export const ROLE_FALLBACK_KIND: Record<SheetRole, string> = {
  component: 'component',
  part: 'part',
  constraint: 'property',
  group: 'group',
};

export interface ResolvedRoles {
  /** 役割 → 実 kind id。 */
  kinds: Record<SheetRole, string>;
  /** その役割が **behaviors 宣言で解決された**か（false＝フォールバックに落ちた）。
      画面はこれを見て「タグがまだ無い」と「そもそも役割が宣言されていない」を
      別の言葉で語る——両者は利用者がやることが違うので、同じ文言にしない。 */
  declared: Record<SheetRole, boolean>;
}

/** その役割の**表示上の呼び名**。宣言があればその kind のラベル、無ければ空文字。
 *
 *  ⚠️ **画面に役割名を書くときは必ずここを通す**（`01KYCC2THS5RX3HB27SQGFWSA5`：
 *  役割はリテラル kind id 固定でなく宣言で決まる以上、呼び名も config が決める）。
 *
 *  宣言が無いときに空文字を返すのは、フォールバックで当たったリテラル id
 *  （`component` 等）が**プロジェクトの名付けたものではない**ため。呼び名として
 *  使うと、生の kind id を画面に出すことになる（`01KYCC2TF3NW3JRSSRK9ZHN078`）。
 *  呼び名が無いときに画面が何と言うかは、呼び出し側が決める。
 *
 *  ⚠️ この判定は `lookups` の中に置いていた。**そこに置くと、値としては正しくても
 *  「宣言の有無を見ない」変異が何にも落ちない**（描画される面がその差を見せない
 *  ときがあるため）。判定をここへ出して、入力に対する答えとして検査できるようにした。 */
export function roleLabel(resolved: ResolvedRoles, role: SheetRole, tagKindLabels: Record<string, string> | undefined): string {
  if (!resolved.declared[role]) return '';
  return (tagKindLabels && tagKindLabels[resolved.kinds[role]]) || resolved.kinds[role];
}

/** config.tagKinds から役割 → 実 kind id を解決する。複数該当時は最初の1つ。 */
export function resolveRoleKinds(tagKinds: KindDecl[] | undefined): ResolvedRoles {
  const kinds = { ...ROLE_FALLBACK_KIND };
  const declared: Record<SheetRole, boolean> = { component: false, part: false, constraint: false, group: false };
  for (const role of Object.keys(ROLE_BEHAVIOR) as SheetRole[]) {
    const marker = ROLE_BEHAVIOR[role];
    for (const decl of tagKinds || []) {
      const o = kindDeclObject(decl);
      if (o.behaviors && o.behaviors.includes(marker)) {
        kinds[role] = o.id;
        declared[role] = true;
        break; // 複数該当時は最初の1つ
      }
    }
  }
  return { kinds, declared };
}
