# Web UI E2E カバレッジ

この文書は、`apps/web/e2e` の既存 Playwright E2E と `docs/ui` の画面・フロー仕様の対応関係を整理する。

目的は、E2E を画面文言のスナップショットとして増やすことではなく、**主要な画面遷移・操作・状態・時間同期がどの仕様を保証しているか追跡できる状態にすること**である。

## 1. 対象

- `apps/web/e2e/stream-library.spec.ts`
- `apps/web/e2e/reservations.spec.ts`
- `docs/ui/navigation.md`
- `docs/ui/screens/stream-list.md`
- `docs/ui/screens/stream-workspace.md`
- `docs/ui/screens/reservations.md`

## 2. 仕様 ID の付与方針

既存テストには、以下の表で `SCR-*` / `FLW-*` を対応付ける。

この対応表を、既存 E2E に対する仕様 ID の付与として扱う。

テスト名そのものへ ID を埋め込むことは必須としない。1つの既存シナリオが複数の Screen / Flow を横断するため、無理に1 IDへ限定すると意図が分かりにくくなるためである。

新規 E2E を追加する場合は、対象が明確なものについて次の形式を推奨する。

```ts
test("FLW-001: opens a stream from the stream list", ...)
```

複数仕様を横断するシナリオでは、テスト近傍のコメントまたは本対応表で関連 ID を示してよい。

## 3. 既存 E2E と仕様の対応

### `stream-library.spec.ts`

| Existing test | 対応仕様 | カバレッジ | 備考 |
|---|---|---|---|
| `registers an ended stream, restores it, and opens it from the library` | `SCR-001`, `SCR-002`, `FLW-001`, `FLW-002` | Partial | Preview → 登録 → Workspace と、一覧から Workspace を開く遷移を保証する。現行 UI 固有の見出し・配置への依存は #46 で差分として扱う |
| `reopens a preview from history without entering the URL again` | `SCR-001` | Legacy / supplemental | Preview 履歴は現行機能だが、`stream-list.md` の必須 Acceptance Criteria ではない。維持要否は #46 で判断する |
| `keeps the main workflow reachable while panels are toggled on mobile` | `SCR-001`, `SCR-002` | Legacy / supplemental | 現行3ペイン UI 固有の振る舞いを検証している。新 UI 仕様の必須構造ではないため、#46 で残置・修正・削除を判断する |
| `starts, restores, retries, and browses a chat collection` | `SCR-002` | Partial | Collection の未実施・処理中・失敗・Retry・成功後の Chat 利用を検証する。成功後の Collection UI 縮小までは未検証 |
| `keeps chat synchronized with playback and seeks from a message` | `SCR-002` | Covered (existing behavior) | Player → Chat current state と Chat message → Player seek の双方向同期を検証する |
| `searches chat and seeks playback from a keyboard-selected result` | `SCR-002` | Covered (existing behavior) | Chat Search result → Player / Chat list の時刻同期と Keyboard 操作を検証する |
| `keeps collected chat available when YouTube embedding fails` | `SCR-002` | Covered (existing behavior) | Player 埋め込み不可でも Chat 分析を継続し、YouTube への代替導線を提供することを検証する |

### `reservations.spec.ts`

| Existing test | 対応仕様 | カバレッジ | 備考 |
|---|---|---|---|
| `creates and cancels a supported reservation` | `SCR-003`, `SCR-004`, `FLW-006` | Partial | 予約作成 → 独立した予約詳細への遷移と、予約詳細での Cancel 操作を検証する |
| `follows automatic collection through completion to stream detail` | `SCR-003`, `SCR-004`, `SCR-002`, `FLW-006`, `FLW-008` | Covered (flow) | 予約作成 → 詳細 → 完了 → 対応する Workspace / Timeline を開く主要フローを検証する |

## 4. Navigation flow カバレッジ

| Flow | 内容 | 現状 | Existing E2E |
|---|---|---|---|
| `FLW-001` | 配信一覧 → Workspace / Timeline | Partial | `registers an ended stream...` |
| `FLW-002` | Preview / 登録 → Workspace / Timeline | Covered (flow) | `registers an ended stream...` |
| `FLW-003` | Primary navigation → Reservations | Not covered | — |
| `FLW-004` | Primary navigation → Streams | Not covered | — |
| `FLW-005` | 予約一覧 → 予約詳細 | Not covered | 既存テストは新規作成から詳細へ遷移するため、一覧行選択は未検証 |
| `FLW-006` | 予約作成 → 予約詳細 | Covered | reservations 2 tests |
| `FLW-007` | 予約詳細 → 予約一覧 | Not covered | — |
| `FLW-008` | 完了予約 → Workspace / Timeline | Covered | `follows automatic collection...` |

## 5. UI仕様上重要だが未検証の項目

以下は UI 仕様上重要だが、現行 E2E では未検証である。

### SCR-001 配信一覧

- 配信追加 UI が通常時は折りたたまれている
- `配信を追加` 操作で同一画面内に入力 UI が展開される
- Preview が登録前に必須である
- 一覧が大型 Card grid ではなく List / Table 形式である
- 一覧に以下が表示される
  - タイトル
  - チャンネル
  - 配信日時
  - 配信時間
  - 配信状態
  - 収集状態
  - チャット件数
- Hero / marketing copy / decorative eyebrow text が恒常表示されない
- Loading / Empty / Error が意味上区別される

### SCR-002 配信ワークスペース / Timeline

- Metadata 詳細が info action から Dialog で開く
- Chat activity が時間 × チャット数の棒グラフで表示される
- 集計単位 5秒 / 10秒 / 30秒を選択でき、初期値が10秒である
- 現在再生位置に対応する棒が active state になる
- 棒グラフ選択 → Player seek → Chat list 同期
- 集計単位変更時に Player の再生位置を維持する
- Player 再生に合わせた Chat message list の自動スクロール
- 手動スクロールで自動追従が一時停止する
- `再生位置に戻る` 操作で自動追従を再開する
- Collection 成功後に Collection UI が主領域を圧迫しない
- Timeline 専用 route / view selector が存在しない

### SCR-003 予約一覧

- 予約作成 UI が通常時は折りたたまれている
- Active / History が分離されている
- Active が主表示対象になっている
- 一覧に以下が表示される
  - 配信タイトルまたは識別情報
  - 配信予定日時
  - 予約状態
  - 次回確認日時
  - エラー有無
- History に `completed / failed / canceled` が分類される
- Hero / catch copy / decorative eyebrow text が恒常表示されない

### SCR-004 予約詳細

- `FLW-007` に対応する予約一覧へ戻る導線
- Loading / Error / Not found の状態差
- Monitoring error / Collection error の表示
- Cancel 不可状態で Cancel action が表示されないことの網羅

## 6. E2E追加の優先順位

#46 で現行実装との差分を整理し、実装修正 Issue を起票した後、E2E はその修正 Issue と同じスコープで追加する。

優先順位は次のとおり。

1. 主要画面遷移 (`FLW-*`)
2. Workspace / Timeline の時刻同期
3. List / Table、Active / History 等の主要情報構造
4. Loading / Empty / Error 等の意味上重要な状態
5. 重要な Forbidden elements

見出し文言や説明文の完全一致を大量にテストして、コピー変更だけで壊れる E2E を増やさない。

Forbidden elements を検証する場合も、個々の文章をすべて blacklist 化するのではなく、仕様上重要な構造（例: Hero が存在しない、登録 UI が初期展開されない）を優先して検証する。

## 7. #46 への引き継ぎ

現行 E2E の一部は、現在の UI 実装を正しくテストしている一方、新しく確定した UI 仕様とは一致しない。

特に以下は、E2E を先に書き換えるのではなく #46 で「仕様と現行実装の差分」として扱う。

- `/streams` の Hero / 常時表示登録フォーム / Card系レイアウト
- 3ペイン・パネル開閉を前提とするテスト
- `/reservations` の常時表示予約フォーム / Hero
- Chat activity graph 未実装
- Metadata Dialog 未実装
- Chat list auto-follow pause/resume 未実装

#45 ではこれらを無理に新仕様へ合わせて failing test にしない。実装差分を明確化した後、対応する実装 Issue で E2E を更新・追加する。
