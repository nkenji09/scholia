import { describe, it, expect } from 'vitest';
import { resolveRoleKinds, roleLabel, ROLE_FALLBACK_KIND } from './roleKinds';
import type { KindDecl } from './types';

// 概要ビューの「役割 → 実 kind id」の解決を、**入力（config.tagKinds）に対する
// 答え**として検査する。画面を起こしてソースを覗く形ではないので、同じ意味を別の
// 綴りで書き直しても答えが変われば落ちる（CLAUDE.md「配線ガードの書き方」1・2）。
//
// **何に落ちるか**: 宣言の読み違い（マーカー名・複数該当の順序・宣言の有無の判定）、
// フォールバック先の取り違え。
// **何に落ちないか**: 解決した kind id を画面が実際に使っているか（それは
// renderWiring.test.tsx の描画ガードが見る）。この file は解決だけを見る。

const decl = (...ds: KindDecl[]) => ds;

describe('役割 kind の解決（01KYCC2THS5RX3HB27SQGFWSA5: リテラル id 固定でなく宣言で決める）', () => {
  it('宣言が1つも無ければ慣用 id へフォールバックし、「宣言された」とは言わない', () => {
    const r = resolveRoleKinds(decl('requirement', 'concern', 'subject'));
    expect(r.kinds).toEqual(ROLE_FALLBACK_KIND);
    expect(r.declared).toEqual({ component: false, part: false, constraint: false, group: false });
  });

  it('config が undefined でも落ちずにフォールバックする', () => {
    expect(resolveRoleKinds(undefined).kinds).toEqual(ROLE_FALLBACK_KIND);
  });

  it('component 概念を別 id（subject）で表しても、その id が役割 component になる', () => {
    const r = resolveRoleKinds(decl('requirement', { id: 'subject', behaviors: ['component'] }));
    expect(r.kinds.component).toBe('subject');
    expect(r.declared.component).toBe(true);
    // 宣言していない役割は巻き添えで動かない。
    expect(r.kinds.part).toBe('part');
    expect(r.declared.part).toBe(false);
  });

  it('4つの役割それぞれが独立に宣言できる（constraint の既定は property）', () => {
    const r = resolveRoleKinds(
      decl(
        { id: 'mod', behaviors: ['component'] },
        { id: 'piece', behaviors: ['part'] },
        { id: 'rule', behaviors: ['constraint'] },
        { id: 'folder', behaviors: ['group'] },
      ),
    );
    expect(r.kinds).toEqual({ component: 'mod', part: 'piece', constraint: 'rule', group: 'folder' });
    expect(ROLE_FALLBACK_KIND.constraint).toBe('property');
  });

  it('1つの kind が複数の役割を兼ねられる', () => {
    const r = resolveRoleKinds(decl({ id: 'subject', behaviors: ['component', 'part'] }));
    expect(r.kinds.component).toBe('subject');
    expect(r.kinds.part).toBe('subject');
  });

  it('同じ役割を複数の kind が宣言したら、宣言順で最初の1つを採る', () => {
    const r = resolveRoleKinds(decl({ id: 'first', behaviors: ['component'] }, { id: 'second', behaviors: ['component'] }));
    expect(r.kinds.component).toBe('first');
  });

  it('axis の宣言は役割解決に混ざらない（別の behaviors 値と取り違えない）', () => {
    const r = resolveRoleKinds(decl({ id: 'axis', behaviors: ['axis'] }, { id: 'subject', behaviors: ['component'] }));
    expect(r.kinds.component).toBe('subject');
    expect(r.kinds.group).toBe('group');
  });

  it('behaviors を持たない object 宣言は、string 宣言と同じく役割を持たない', () => {
    const r = resolveRoleKinds(decl({ id: 'subject', label: 'コンポーネント', description: '主題' }));
    expect(r.kinds.component).toBe('component');
    expect(r.declared.component).toBe(false);
  });
});

// 呼び名の解決。⚠️ **この判定は元は lookups の中にあり、そこでは「宣言の有無を
// 見ない」変異が何にも落ちなかった**——値としては変わるのに、その差を見せる面が
// 描かれない状態があったため（レビュアの変異1件がそこを通った）。判定をここへ
// 出したので、入力に対する答えとして落とせる。
describe('役割の呼び名（画面に literal で書かない・01KYCC2THS5RX3HB27SQGFWSA5）', () => {
  it('宣言があれば、その kind の設定ラベルを返す', () => {
    const r = resolveRoleKinds(decl({ id: 'subject', behaviors: ['component'] }));
    expect(roleLabel(r, 'component', { subject: 'コンポーネント' })).toBe('コンポーネント');
  });

  it('宣言が無ければ空文字（フォールバック先の生の kind id を呼び名にしない）', () => {
    const r = resolveRoleKinds(decl('requirement', 'concern', 'subject'));
    // ⚠️ ラベルが**設定に在っても**返さない。フォールバックで当たった `component` は
    // このプロジェクトが名付けたものではない（01KYCC2TF3NW3JRSSRK9ZHN078）。
    expect(roleLabel(r, 'component', { component: 'コンポーネント' })).toBe('');
  });

  it('宣言はあるがラベルが無ければ、その kind id を返す', () => {
    const r = resolveRoleKinds(decl({ id: 'subject', behaviors: ['component'] }));
    expect(roleLabel(r, 'component', {})).toBe('subject');
    expect(roleLabel(r, 'component', undefined)).toBe('subject');
  });

  it('役割ごとに別々に解決する（別の役割のラベルを取ってこない）', () => {
    const r = resolveRoleKinds(decl({ id: 'subject', behaviors: ['component'] }, { id: 'piece', behaviors: ['part'] }));
    const labels = { subject: 'コンポーネント', piece: '構成要素' };
    expect(roleLabel(r, 'component', labels)).toBe('コンポーネント');
    expect(roleLabel(r, 'part', labels)).toBe('構成要素');
  });
});
