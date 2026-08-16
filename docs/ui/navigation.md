# Web UI 画面遷移

この文書は `apps/web` の主要画面とユーザーフローの正本である。

個別画面の詳細な構造・表示要素・状態は `docs/ui/screens/*.md` で定義し、横断的な設計原則は `docs/ui/principles.md` に従う。

## 1. 主要画面

| ID | 画面 | URL | 主目的 |
|---|---|---|---|
| `SCR-001` | 配信一覧 | `/streams` | 対象配信を登録し、既存配信を選択する |
| `SCR-002` | 配信ワークスペース | `/streams/:streamId` | 対象配信の収集・確認・分析を行う |
| `SCR-003` | 予約一覧 | `/reservations` | 自動収集予約を作成し、既存予約を確認する |
| `SCR-004` | 予約詳細 | `/reservations/:reservationId` | 予約状態と詳細を確認し、必要な操作を行う |

### 現在のスコープ

本書では現行 Web UI の主要画面を正本化する。

Timeline / Location / PC view の詳細な URL・Workspace 内での関係・ビュー間遷移は #44 で決定する。現時点では推測で追加しない。

## 2. グローバルナビゲーション

Primary navigation では以下を提供する。

| Action | Destination |
|---|---|
| Streams | `SCR-001` `/streams` |
| Reservations | `SCR-003` `/reservations` |

実装者は仕様にないグローバルナビゲーション項目を追加しない。

## 3. ユーザーフロー

### FLW-001 配信一覧から配信ワークスペースを開く

- Entry: `SCR-001` 配信一覧
- Action: 一覧から対象配信を選択する
- Destination: `SCR-002` 配信ワークスペース
- URL: `/streams/:streamId`
- 許可される中間状態: なし
- 禁止される中間 UI:
  - 確認画面
  - Preview modal
  - 別の詳細画面
  - 遷移確認 Dialog

対象選択後は直接 Workspace へ遷移する。

### FLW-002 YouTube URL から配信を登録する

- Entry: `SCR-001` 配信一覧
- Action:
  1. YouTube URL を入力する
  2. Preview を実行する
  3. Preview 内容を確認して登録する
- Destination: `SCR-002` 配信ワークスペース
- URL: `/streams/:streamId`
- 許可される中間状態:
  - Preview loading
  - Preview result
  - Registering
  - Validation / API error
- 禁止される中間 UI:
  - Preview 専用ページ
  - 登録完了専用ページ
  - 不要な確認 Dialog

登録成功後は直接登録対象の Workspace へ遷移する。

### FLW-003 Primary navigation から予約一覧を開く

- Entry: Web UI 内の主要画面
- Action: Primary navigation の Reservations を選択する
- Destination: `SCR-003` 予約一覧
- URL: `/reservations`
- 許可される中間状態: なし
- 禁止される中間 UI:
  - 確認 Dialog
  - 中間メニュー画面

### FLW-004 Primary navigation から配信一覧を開く

- Entry: Web UI 内の主要画面
- Action: Primary navigation の Streams を選択する
- Destination: `SCR-001` 配信一覧
- URL: `/streams`
- 許可される中間状態: なし
- 禁止される中間 UI:
  - 確認 Dialog
  - 中間メニュー画面

### FLW-005 予約一覧から予約詳細を開く

- Entry: `SCR-003` 予約一覧
- Action: 予約一覧から対象予約を選択する
- Destination: `SCR-004` 予約詳細
- URL: `/reservations/:reservationId`
- 許可される中間状態: なし
- 禁止される中間 UI:
  - 予約詳細 Modal
  - 予約詳細 Drawer
  - 遷移確認 Dialog

予約詳細は独立画面として扱う。

### FLW-006 予約を作成する

- Entry: `SCR-003` 予約一覧
- Action:
  1. YouTube URL を入力する
  2. 予約作成を実行する
- Destination: `SCR-004` 作成した予約の詳細
- URL: `/reservations/:reservationId`
- 許可される中間状態:
  - Submitting
  - Validation / API error
- 禁止される中間 UI:
  - 予約作成完了専用画面
  - 不要な確認 Dialog

予約作成成功後は、作成された予約の詳細画面へ直接遷移する。

### FLW-007 予約詳細から予約一覧へ戻る

- Entry: `SCR-004` 予約詳細
- Action: 予約一覧へ戻る操作を行う
- Destination: `SCR-003` 予約一覧
- URL: `/reservations`
- 許可される中間状態: なし
- 禁止される中間 UI: なし

## 4. 遷移表

| From | Action | To |
|---|---|---|
| `SCR-001` 配信一覧 | 配信を選択 | `SCR-002` 配信ワークスペース |
| `SCR-001` 配信一覧 | 配信を登録 | `SCR-002` 配信ワークスペース |
| 任意の主要画面 | Streams | `SCR-001` 配信一覧 |
| 任意の主要画面 | Reservations | `SCR-003` 予約一覧 |
| `SCR-003` 予約一覧 | 予約を選択 | `SCR-004` 予約詳細 |
| `SCR-003` 予約一覧 | 予約を作成 | `SCR-004` 予約詳細 |
| `SCR-004` 予約詳細 | 一覧へ戻る | `SCR-003` 予約一覧 |

## 5. 遷移実装の原則

- 本書にない画面遷移を実装者判断で追加しない
- 本書にない中間画面・Modal・Drawer・確認 Dialog を追加しない
- Modal と独立画面を実装都合で置き換えない
- URL を持つ画面は reload / direct access でも同じ画面として成立させる
- Browser back / forward によって URL と表示画面が不整合にならないようにする
- 未定義の遷移が必要になった場合は、実装より先に本書を更新する

## 6. 詳細仕様との関係

各画面仕様は、本書の `SCR-*` と `FLW-*` を参照する。

後続の画面仕様では最低限、以下の対応を明記する。

- Entry に対応する `FLW-*`
- Primary actions が発生させる `FLW-*`
- 画面固有の Loading / Empty / Error state
- 画面内に表示する要素と禁止する要素

本書と個別画面仕様が矛盾する場合は、`docs/ui/principles.md` に定義された正本の優先順位に従う。
