# SCR-003 / SCR-004 Reservations UI仕様

この文書は、`/reservations` の予約一覧画面と `/reservations/:reservationId` の予約詳細画面の正本である。

横断ルールは `docs/ui/principles.md`、画面遷移は `docs/ui/navigation.md` を参照する。

## 1. Purpose

Reservations UI の主目的は、**現在の自動収集予約・監視状態を確認し、対応が必要な予約を把握すること**である。

新しい収集予約の作成も本画面で行うが、主目的ではなく補助操作として扱う。

優先順位は次のとおり。

1. 現在進行中の予約・監視状態を確認する
2. エラーや注意が必要な予約を識別する
3. 必要に応じて予約詳細を確認・操作する
4. 新しい収集予約を作成する
5. 完了・失敗・キャンセル済みの履歴を確認する

## 2. 対象画面

| Screen ID | 画面 | URL | 主目的 |
|---|---|---|---|
| `SCR-003` | 予約一覧 | `/reservations` | Active な予約状態を一覧で確認し、新しい予約を作成する |
| `SCR-004` | 予約詳細 | `/reservations/:reservationId` | 1件の予約について状態・時刻・エラー・可能な操作を確認する |

予約詳細は Modal / Drawer ではなく独立画面として扱う。

## 3. Entry

### SCR-003 予約一覧

主な Entry は以下とする。

- Primary navigation の `Reservations` から遷移する (`FLW-003`)
- 予約詳細から一覧へ戻る (`FLW-007`)
- `/reservations` への direct access / reload

### SCR-004 予約詳細

主な Entry は以下とする。

- 予約一覧から対象予約を選択する (`FLW-005`)
- 新しい予約の作成成功後に遷移する (`FLW-006`)
- `/reservations/:reservationId` への direct access / reload

## 4. Reservation state の分類

予約状態は、一覧上では `Active` と `History` に分類する。

### Active

現在進行中であり、予約一覧の主表示対象とする。

- `scheduled` — 配信待ち
- `monitoring` — 監視中
- `live` — ライブ配信中
- `waiting_for_archive` — アーカイブ待ち
- `collecting` — 収集中

### History

終端状態であり、Active とは分離して履歴として表示する。

- `completed` — 完了
- `failed` — 失敗
- `canceled` — キャンセル済み

History は削除せず、過去の予約結果を確認できる状態を維持する。

## 5. SCR-003 Information hierarchy

予約一覧画面の情報階層は次の順とする。

1. Active reservations
2. Active reservations の状態・エラー有無
3. 新規予約作成操作
4. History
5. Loading / Empty / Error 等の状態表示

新規予約フォームや説明文によって Active reservations の一覧領域を圧迫しない。

## 6. SCR-003 Layout

基本構造は以下とする。

```text
Header
├── Brand
└── Primary navigation
    ├── Streams
    └── Reservations

Main
├── Page heading
├── Create reservation action
│   └── [展開時]
│       ├── YouTube URL input
│       └── Create action
├── Active reservations
│   └── Reservation list / table
└── History
    └── Reservation list / table
```

### 新規予約作成 UI

新規予約作成 UI は常時展開しない。

- 通常時は `収集を予約` に相当する補助操作のみ表示する
- ユーザーが予約作成を開始した場合に入力 UI を同一画面内で展開する
- 別の予約作成専用画面へ遷移しない
- 予約作成 UI が展開されても Active reservations を主コンテンツとして維持する
- 仕様にない確認 Dialog や Step UI を追加しない

## 7. SCR-003 Reservation list

一覧は、複数予約の状態を走査しやすい**情報密度の高い List / Table 形式**を基本とする。

大型カードを予約ごとに並べる構成は採用しない。

### 最低限の表示項目

各予約について、最低限次を表示する。

| 項目 | 意味 |
|---|---|
| 配信タイトルまたは識別情報 | 取得済みの場合は配信タイトルを優先し、取得できない場合は YouTube Video ID 等の安定した識別情報を表示する |
| 配信予定日時 | YouTube から取得できる予定開始日時。未取得・不明の場合は不明であることを表現する |
| 予約状態 | 配信待ち / 監視中 / ライブ配信中 / アーカイブ待ち / 収集中 / 完了 / 失敗 / キャンセル済み |
| 次回確認日時 | 次に監視処理を行う予定日時。終端状態など値が存在しない場合は適切に非表示または対象外として表現する |
| エラー有無 | 現在または直近の監視・収集エラーが存在するかを識別できる状態 |

### Active reservations

- 画面の主一覧として表示する
- 予約状態を短く識別できること
- エラーがある予約を一覧走査時に識別できること
- 予約を選択すると `SCR-004` へ直接遷移する (`FLW-005`)
- 各行に長い `nextAction` 説明文を常時表示しない

### History

- Active と視覚的・構造的に区別する
- 完了 / 失敗 / キャンセル済みを履歴として確認できる
- History からも予約詳細を開ける
- Active より強い視覚優先度にしない
- 履歴を表示するためだけの別ページは、この仕様では必須としない

### 並び順

Active / History 内の具体的な並び順は本 Issue では固定しない。

必要になった場合は、運用上の優先度や利用実態を踏まえて別途決定する。

## 8. SCR-003 Primary actions

### ACT-RSV-001 予約詳細を開く

- 対象: Active / History の予約
- Action: 対象予約を選択する
- Flow: `FLW-005`
- Destination: `SCR-004`
- URL: `/reservations/:reservationId`

中間 Modal / Drawer / 確認 Dialog を挟まず直接遷移する。

### ACT-RSV-002 予約作成 UI を開く

- 対象: `収集を予約` 操作
- Action: 同一画面内で予約作成 UI を展開する
- Navigation: なし

### ACT-RSV-003 予約を作成する

- 対象: 展開した予約作成 UI
- Action:
  1. YouTube URL を入力する
  2. 予約作成を実行する
- Flow: `FLW-006`
- Destination: 作成した予約の `SCR-004`
- URL: `/reservations/:reservationId`

予約作成成功後は完了専用画面を挟まず、作成した予約の詳細へ直接遷移する。

## 9. SCR-003 Visible elements

通常状態で表示する要素:

- Header
- Primary navigation
- 画面タイトル
- `収集を予約` 操作
- Active reservations
- History
- 各一覧に必要な項目名・状態
- 必要な Loading / Empty / Error 表示

予約作成 UI を展開した場合のみ表示する要素:

- YouTube URL input
- Create action
- Submitting state
- 予約作成に関する Validation / Error

## 10. SCR-003 Forbidden elements

以下を恒常表示しない。

- 大型 Hero section
- Catch phrase
- Marketing copy
- Brand message
- Welcome message
- Tips / Tutorial
- decorative eyebrow text
- 「配信状態を自動で確認し〜」等の恒常的な機能説明文
- 空白を埋めるための説明文
- 大型 Card grid
- 仕様にない CTA
- 予約作成専用ページ
- 予約作成完了専用ページ
- 予約詳細 Modal / Drawer
- 予約選択時の確認 Dialog

現行実装に存在する以下のような表現は、UI仕様上の必須要素としない。

- `自動収集` の decorative eyebrow
- `配信終了前に、収集を予約。` のキャッチコピー
- `監視キュー` の decorative eyebrow
- 恒常表示される機能説明文

## 11. SCR-003 States

### Loading

予約一覧取得中であることを明示する。

- Loading と Empty を混同しない
- 読み込み完了前に0件と断定しない

### Ready

Active / History をそれぞれ表示する。

### Empty — Active 0件

現在進行中の予約が存在しないことを簡潔に表示する。

History が存在する場合は History を引き続き表示する。

必要であれば `収集を予約` 操作へ誘導してよいが、長い説明文は表示しない。

### Empty — History 0件

履歴が存在しないことを簡潔に表現する。

履歴セクション自体を非表示にするか、簡潔な Empty 表示にするかは実装上の選択としてよい。

### Create reservation expanded

予約作成 UI が展開された状態。

- URL を入力できる
- Active reservations は引き続き主コンテンツとして存在する

### Submitting

予約作成処理中であることを示し、重複送信を防止する。

### Error

一覧取得エラーと予約作成エラーを区別する。

- API 内部情報をそのまま表示しない
- 入力値を修正・再試行できる場合は操作可能な状態を維持する
- 一覧取得に失敗した場合、予約が0件であるかのように表示しない

## 12. SCR-004 Purpose

予約詳細画面の目的は、**1件の予約が現在どの状態にあり、次に何が起きるか、ユーザー操作が必要かを確認すること**である。

この画面は予約の状態監視・トラブル確認・許可された操作の実行に使う。

## 13. SCR-004 Information hierarchy

予約詳細の情報階層は次の順とする。

1. 予約対象の識別情報
2. 現在の予約状態
3. 次に予定されている処理 / 次回確認日時
4. エラー・注意事項
5. 必要な主要操作
6. 補助メタデータ

状態名を大型 Hero のように演出せず、状態理解に必要な情報として表示する。

## 14. SCR-004 Layout

基本構造は以下とする。

```text
Main
├── Back to reservations
├── Reservation heading
│   ├── Stream identity
│   └── Reservation state
├── Status / schedule details
├── Issue / error [存在時]
└── Actions
```

詳細画面でもマーケティングコピーや decorative eyebrow を追加しない。

## 15. SCR-004 Visible information

最低限、取得可能な範囲で次を表示する。

- 配信タイトルまたは YouTube Video ID 等の識別情報
- 現在の予約状態
- 配信予定日時
- 次回確認日時
- 監視試行回数
- 監視エラー（存在時）
- 収集エラー（存在時）
- エラーが自動再試行されるかどうか
- YouTube を開く操作
- 予約キャンセル操作（`canCancel` の場合）
- 完了済み Stream を開く操作（`completed` かつ `streamId` がある場合）

内部実装の詳細や生の例外情報を表示しない。

## 16. SCR-004 Primary actions

### ACT-RSV-101 予約一覧へ戻る

- Flow: `FLW-007`
- Destination: `SCR-003`
- URL: `/reservations`

### ACT-RSV-102 YouTube を開く

予約元の YouTube URL を外部で開く。

### ACT-RSV-103 予約をキャンセルする

- `canCancel` が true の場合のみ提供する
- 処理中は重複操作を防止する
- キャンセル成功後は同じ詳細画面上で更新された状態を表示する

キャンセル確認 Dialog の要否は本仕様では新規に要求しない。既存仕様に明示がない場合、実装者判断で追加しない。

### ACT-RSV-104 収集済み Stream を開く

- 条件: `state === completed` かつ `streamId` が存在する
- Destination: `SCR-002` 配信ワークスペース
- URL: `/streams/:streamId`

中間画面を挟まず直接 Workspace へ遷移する。

## 17. SCR-004 States

### Loading

予約詳細取得中であることを明示する。

### Ready

予約状態・予定・利用可能な操作を表示する。

Active な予約については状態更新を反映できること。

### Error — load failure

予約詳細を取得できない場合、取得失敗であることを示す。

存在しない予約と一時的な取得失敗を、API が識別可能な場合は同じ意味として扱わない。

### Error — monitoring / collection issue

予約自体は取得できているが監視・収集処理でエラーが発生している場合は、通常の詳細情報を維持したうえで問題を明示する。

- ユーザー向けに変換されたエラー内容を表示する
- 自動再試行の有無を表示する
- ユーザーが取れる行動がある場合のみ操作を提示する

### Canceling

キャンセル処理中であることを示し、重複操作を防止する。

### Terminal

`completed` / `failed` / `canceled` では不要な監視中表示や次回確認表示を残さない。

## 18. Data availability

予約一覧では次の情報を要求する。

- 配信タイトルまたは識別情報
- 配信予定日時
- 予約状態
- 次回確認日時
- エラー有無

現行 API が一覧取得レスポンスで配信タイトル等を提供していない場合、UI から予約ごとに個別APIやYouTube APIを大量に呼び出して補完することを前提にしない。

必要な場合は API / read model の変更を、現行実装との差分整理時に後続 Issue として扱う。

タイトルが未取得の場合でも予約一覧を利用不能にせず、YouTube Video ID 等の安定した識別情報へフォールバックする。

## 19. Acceptance Criteria

### SCR-003

- [ ] `/reservations` の主目的が「現在の予約・監視状態を確認すること」として実装されている
- [ ] 予約作成 UI は通常時に折りたたまれている
- [ ] `収集を予約` 操作で同一画面内に URL 入力 UI が展開される
- [ ] Active と History が分離されている
- [ ] Active に `scheduled` / `monitoring` / `live` / `waiting_for_archive` / `collecting` が分類される
- [ ] History に `completed` / `failed` / `canceled` が分類される
- [ ] 一覧に配信タイトルまたは識別情報 / 配信予定日時 / 予約状態 / 次回確認日時 / エラー有無が表示される
- [ ] 予約選択後は `FLW-005` に従い予約詳細へ直接遷移する
- [ ] 予約作成成功後は `FLW-006` に従い作成した予約詳細へ直接遷移する
- [ ] Loading / Empty / Submitting / Error が区別されている
- [ ] Hero / catch phrase / decorative eyebrow / 恒常的な機能説明文が表示されていない
- [ ] 仕様にない Modal / Drawer / 中間画面を追加していない

### SCR-004

- [ ] 予約対象と現在状態を識別できる
- [ ] 配信予定日時 / 次回確認日時 / 監視試行回数を取得可能な範囲で確認できる
- [ ] 監視エラーと収集エラーを確認できる
- [ ] エラーの自動再試行有無を確認できる
- [ ] `canCancel` の場合のみキャンセル操作が提供される
- [ ] 完了済みで `streamId` がある場合に配信ワークスペースへ直接遷移できる
- [ ] `FLW-007` に従い予約一覧へ戻れる
- [ ] Loading / Ready / Error / Canceling / Terminal の状態が区別されている

## 20. Out of scope

- 現行 Reservations UI の実装修正
- 一覧に不足するデータを提供する API の実装変更
- Active / History 内の詳細なソート順
- 予約の検索・フィルタ・ページング
- 履歴の保持期間・削除機能
- 一括キャンセル等のバルク操作
- 通知機能

これらが必要になった場合は、別途プロダクト判断または後続 Issue として扱う。
