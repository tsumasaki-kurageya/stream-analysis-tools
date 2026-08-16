# SCR-001 配信一覧 UI仕様

この文書は、`/streams` に表示する配信一覧画面の正本である。

横断ルールは `docs/ui/principles.md`、画面遷移は `docs/ui/navigation.md` を参照する。

## 1. Purpose

本画面の主目的は、**登録済み配信から分析対象を選択すること**である。

新しい配信の登録は本画面で行うが、主目的ではなく補助操作として扱う。

優先順位は次のとおり。

1. 登録済み配信を一覧で比較し、分析対象を選択する
2. 必要に応じて新しい配信を登録する

## 2. Entry

主な Entry は以下とする。

- Primary navigation の `Streams` から遷移する (`FLW-004`)
- 配信ワークスペースから配信一覧へ戻る
- `/streams` への direct access / reload

## 3. Information hierarchy

画面内の情報階層は次の順とする。

1. 配信一覧
2. 配信追加操作
3. 配信追加時の Preview
4. Loading / Empty / Error などの状態表示

配信一覧を画面の主コンテンツとし、配信追加UIや説明文によって一覧領域を圧迫しない。

## 4. Layout

基本構造は以下とする。

```text
Header
├── Brand
└── Primary navigation
    ├── Streams
    └── Reservations

Main
├── Page heading
├── Add stream action
│   └── [展開時]
│       ├── YouTube URL input
│       ├── Preview action
│       └── Preview result
└── Stream list
```

### 配信追加 UI

YouTube URL 登録 UI は常時展開しない。

- 通常時は `配信を追加` に相当する補助操作のみ表示する
- ユーザーが配信追加を開始した場合に入力 UI を展開する
- 入力 UI を展開しても、配信一覧を主コンテンツとして維持する
- 配信追加のために別画面へ遷移しない

## 5. Stream list

配信一覧は**情報密度の高い List / Table 形式**とする。

サムネイル主体の大型カード一覧は採用しない。

一覧から複数配信を比較しやすくし、スクロール量を抑えることを優先する。

### 表示項目

各配信について、最低限次を表示する。

| 項目 | 意味 |
|---|---|
| タイトル | 配信タイトル |
| チャンネル | 配信チャンネル名 |
| 配信日時 | 実際の配信開始日時。未確定の場合は取得可能な予定日時等を適切に表現する |
| 配信時間 | 配信の長さ (`Duration`) |
| 配信状態 | scheduled / live / ended 等の配信ライフサイクル状態 |
| 収集状態 | チャット等の収集処理が未実施・処理中・成功・失敗等のどの状態か |
| チャット件数 | 現在利用可能な収集済みチャット件数 |

### 表示密度

- 1配信あたりの縦方向の占有を抑える
- 一覧比較に不要な長文説明を各行に置かない
- ステータスは一覧走査しやすい短い表現にする
- タイトル以外の補助情報がタイトルより強く見えないようにする

### Thumbnail

Thumbnail は必須表示項目としない。

一覧性を損なわず補助情報として有効な場合のみ、小型表示を許可する。

## 6. Primary actions

### ACT-001 登録済み配信を開く

- 対象: 一覧上の登録済み配信
- Action: 配信を選択
- Flow: `FLW-001`
- Destination: `SCR-002` 配信ワークスペース
- URL: `/streams/:streamId`

対象選択後は中間画面を挟まず、直接配信ワークスペースへ遷移する。

### ACT-002 配信追加 UI を開く

- 対象: `配信を追加` 操作
- Action: 配信追加 UI を展開する
- Navigation: なし

Modal や別画面を新設せず、本画面内で展開する。

### ACT-003 配信を Preview する

- 対象: 展開した配信追加 UI
- Action:
  1. YouTube URL を入力する
  2. Preview を実行する
- Flow: `FLW-002` の中間操作

Preview は登録前に必須とする。

### ACT-004 Preview 内容を確認して登録する

- 対象: Preview result
- Action: 登録を実行する
- Flow: `FLW-002`
- Destination: `SCR-002` 配信ワークスペース
- URL: `/streams/:streamId`

登録成功後は登録完了画面を挟まず、直接配信ワークスペースへ遷移する。

## 7. Preview

Preview は誤った配信を登録することを防ぐため、登録前の必須確認ステップとする。

Preview result には、ユーザーが登録対象を識別するために必要な情報のみ表示する。

最低限、次を表示する。

- タイトル
- チャンネル
- 配信日時または配信状態の確認に必要な情報
- Thumbnail（取得できる場合）

Preview result を独立画面にしない。

Preview result 内に、機能説明・プロモーション文・不要な補助コピーを追加しない。

## 8. Visible elements

通常状態で表示する要素:

- Header
- Primary navigation
- 画面タイトル
- `配信を追加` 操作
- 配信一覧
- 配信一覧に必要な列名 / 項目名
- 各配信の一覧表示項目
- 必要な状態表示

配信追加を展開した場合のみ表示する要素:

- YouTube URL input
- Preview action
- Preview result
- 登録 action
- Preview / 登録処理に関する Loading / Error state

## 9. Forbidden elements

以下を本画面に恒常表示しない。

- 大型 Hero section
- Marketing copy
- Catch phrase
- Brand message
- Welcome message
- Tips
- Tutorial
- decorative eyebrow text
- 「この画面では〜できます」のような機能説明文
- 空白を埋める目的の補助文
- サムネイル主体の大型 Card grid
- 仕様にない CTA
- 配信選択時の確認 Dialog
- 配信選択時の Preview Modal
- 配信登録完了専用画面
- 配信追加専用ページ

現行実装に存在する以下の表現は、UI仕様上は不要とする。

- `YouTube stream workspace`
- `Save the streams worth returning to.`
- それらに付随する恒常的な説明文
- `Your collection` のような decorative eyebrow text

## 10. States

### Loading

配信一覧の取得中であることを明示する。

- Empty state と混同しない
- 読み込み完了前に「0件」と表示しない

### Ready

取得済み配信を List / Table 形式で表示する。

### Empty

登録済み配信が0件であることを簡潔に表示する。

Empty state では、必要であれば `配信を追加` 操作へ誘導してよい。

ただし、長いオンボーディング説明やマーケティングコピーは表示しない。

### Add stream expanded

配信追加 UI が展開された状態。

- URL 入力可能
- 配信一覧は引き続き画面上の主コンテンツとして存在する

### Preview loading

Preview API の処理中であることを示し、重複送信を防止する。

### Preview ready

取得した配信情報を確認でき、登録操作が可能な状態。

### Registering

登録処理中であることを明示し、重複登録操作を防止する。

### Error

一覧取得・Preview・登録のエラーを、発生した操作に対応する位置で表示する。

- API 内部情報をそのまま表示しない
- 再試行可能な場合は再実行できる状態を維持する
- 一覧取得エラーと配信追加エラーを同一の意味として扱わない

## 11. Data availability

本仕様は、一覧で次の情報を表示できることを要求する。

- 配信日時
- 配信時間
- 配信状態
- 収集状態
- チャット件数

現行 API がこれらを一覧取得レスポンスで提供していない場合、UI から個別 API を大量に呼び出して補完することを前提にしない。

必要な API / read model の変更は、現行実装との差分整理時に後続 Issue として扱う。

## 12. Acceptance Criteria

- [ ] `/streams` の主目的が「登録済み配信から分析対象を選択すること」として実装されている
- [ ] 配信追加 UI は通常時に折りたたまれている
- [ ] `配信を追加` 操作で同一画面内に YouTube URL 入力 UI が展開される
- [ ] 配信登録前に Preview が必須である
- [ ] Preview は独立画面や Modal ではなく同一画面内に表示される
- [ ] 配信一覧は大型 Card grid ではなく List / Table 形式である
- [ ] 一覧にタイトル / チャンネル / 配信日時 / 配信時間 / 配信状態 / 収集状態 / チャット件数が表示される
- [ ] 配信選択後は `FLW-001` に従い直接配信ワークスペースへ遷移する
- [ ] 配信登録後は `FLW-002` に従い直接配信ワークスペースへ遷移する
- [ ] Loading / Empty / Error / Preview / Registering の状態が区別されている
- [ ] Hero / marketing copy / decorative eyebrow text 等の禁止要素が恒常表示されていない
- [ ] 仕様にない Modal / Dialog / 中間画面を追加していない

## 13. Out of scope

- 現行 `/streams` の実装修正
- 一覧表示に必要な API の実装変更
- 配信ワークスペースの詳細 UI 仕様
- Timeline / Location / PC view のナビゲーション
- 並び替え、フィルタ、検索、ページング等の追加機能

これらが必要になった場合は、別途プロダクト判断または後続 Issue として扱う。
