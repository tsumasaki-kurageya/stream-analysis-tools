# Web UI 仕様と現行実装の差分

この文書は Issue #46 の成果物として、確定済み Web UI 仕様と `apps/web` の現行実装の差分を整理する。

目的は UI の好みを列挙することではなく、正本仕様に対してどの実装が一致しておらず、どの単位で修正・検証すべきかを明確にすることである。

## 1. 比較対象

正本として以下を参照する。

- `docs/ui/principles.md`
- `docs/ui/navigation.md`
- `docs/ui/screens/stream-list.md`
- `docs/ui/screens/stream-workspace.md`
- `docs/ui/screens/reservations.md`
- `docs/ui/e2e-coverage.md`

現行実装として主に以下を確認した。

- `apps/web/src/App.tsx`
- `apps/web/src/ReservationsPage.tsx`
- `apps/web/src/YouTubePlayer.tsx`
- `apps/web/src/api/client.ts`
- `apps/web/src/api/generated/v1.ts`
- `apps/web/e2e/stream-library.spec.ts`
- `apps/web/e2e/reservations.spec.ts`

## 2. 判定ルール

差分は、次の優先順位に従って判定する。

1. 明示された要求 / Acceptance Criteria
2. `docs/ui/screens/*.md`
3. `docs/ui/navigation.md`
4. `docs/ui/principles.md`
5. E2E
6. 現行実装

現行実装に存在する UI が仕様に存在しない場合、既存であること自体を仕様根拠とはしない。

特に以下の原則を重視する。

- Web UI は marketing / landing page ではなく task-oriented analysis interface とする
- 主目的に必要な情報領域を優先する
- Hero / catch copy / decorative eyebrow / 恒常的な機能説明を追加しない
- 仕様にない Sidebar / Card / Modal / Help UI 等を実装者判断で追加しない
- 未定義の遷移や UI を実装で先行させない

## 3. 結論

現行実装は、主要 URL と主要フローの多くは維持できている一方、Issue #36 で導入された全画面共通3ペイン UI が、新しく正本化した画面ごとの Information hierarchy と大きくずれている。

特に修正優先度が高いのは以下である。

1. 全画面共通 App shell の3ペイン前提を解除する
2. `/streams` を一覧中心の `SCR-001` に再構成する
3. `/streams/:streamId` を Timeline として必要な Chat activity / metadata / follow behavior を実装する
4. `/reservations` を Active monitoring 中心の `SCR-003` に再構成する
5. UI に必要な一覧集計・Timeline集計 read model を API 側に追加する

## 4. 差分一覧

| Gap ID | 対象 | 仕様参照 | 現行実装との差分 | 修正 Issue |
|---|---|---|---|---|
| `GAP-001` | 全画面 | UI principles / `SCR-001`〜`SCR-004` | `App.tsx` が全画面を左Panel / Main / 右Panel の3ペインへ固定し、各画面固有の情報階層をグローバルShellが支配している | #54 |
| `GAP-002` | `SCR-001` | Stream list §Purpose / Information hierarchy | 登録済み配信一覧がMainではなく左サイドバーに置かれ、一覧選択が画面の主目的として表現されていない | #54, #55 |
| `GAP-003` | `SCR-001` | Stream list §配信追加UI | YouTube URL入力フォームが右Panelに常時表示され、明示操作で展開する補助操作になっていない | #55 |
| `GAP-004` | `SCR-001` | Stream list §Forbidden elements / UI principles | Hero、catch copy、decorative eyebrow、welcome説明、仕様にないPreview historyが恒常表示されている | #55 |
| `GAP-005` | `SCR-001` | Stream list §表示項目 | 一覧はタイトル / チャンネル中心で、配信日時 / 配信時間 / 配信状態 / 収集状態 / チャット件数を一覧比較できない | #55, #56 |
| `GAP-006` | `SCR-001` data | Stream list §Data availability | `GET /v1/streams` はStream metadataのみで、収集状態 / チャット件数を返さない | #56 |
| `GAP-007` | `SCR-002` | Workspace §Workspace header / Metadata detail | 配信日 / Duration / YouTube Video ID等が常時表示され、info action / Metadata Dialogがない | #57 |
| `GAP-008` | `SCR-002` | Workspace §Collection | Collection成功後も processed / skipped / attempt の大型summaryが残り、分析領域より強い面積を占有する | #57 |
| `GAP-009` | `SCR-002` | Workspace §Visible elements / Timeline | Chat Search / Chat listがグローバル右Panel開閉に依存し、Timelineの主要分析領域を丸ごと非表示にできる | #54, #57 |
| `GAP-010` | `SCR-002` | Workspace §Chat activity | 時間 × チャット件数の棒グラフ、5秒 / 10秒 / 30秒selector、active bucket、graph -> Player seekが存在しない | #58 |
| `GAP-011` | `SCR-002` data | Workspace §Data / API requirements | 配信全体の時間bucket別Chat countを取得するAPI/read modelがない | #58 |
| `GAP-012` | `SCR-002` | Workspace §Chat message list / 自動追従 | current messageへの `aria-current` は更新するが、Player再生位置への自動スクロール、手動scroll時のpause、resume actionがない | #59 |
| `GAP-013` | `SCR-003` | Reservations §Information hierarchy / Create reservation | 予約作成フォームがHero内で常時表示され、予約監視一覧より先に強調されている | #60 |
| `GAP-014` | `SCR-003` | Reservations §Reservation list / Forbidden elements | Active / Historyが未分離で、行は状態 / Video ID / 長いnextAction説明中心。配信予定日時 / 次回確認日時 / エラー有無を一覧確認できない | #60 |
| `GAP-015` | `SCR-003` | Reservations §Forbidden elements | `自動収集` / `配信終了前に、収集を予約。` / 説明文 / `監視キュー` 等のHero・catch copy・decorative eyebrowが存在する | #60 |
| `GAP-016` | `SCR-004` | Reservations §Information hierarchy / Terminal | decorative eyebrowがあり、状態名をHero的に強調している。Terminal状態でも次回確認欄を残す構造になっている | #60 |
| `GAP-017` | E2E | `docs/ui/e2e-coverage.md` | 現行E2Eは旧Hero / 3ペイン等を前提とする箇所があり、新仕様で必要なGraph / follow / Active-History等は未検証 | #54〜#60 各修正Issue内で更新 |

## 5. 画面別詳細

### 5.1 App shell / navigation

#### 差分

現行 `App.tsx` は Primary navigation に加えて、全ルート共通で以下を持つ。

- 左Panel開閉
- 右Panel開閉
- Streams側のライブラリSidebar
- Reservations側の `収集メニュー` Sidebar
- Streams側の `操作パネル`
- Reservations側の説明用右Panel

これらは `SCR-001`〜`SCR-004` の正本で共通Shell要素として要求していない。

特に `/streams` の一覧と新規追加が左右Panelへ分割されることで、`SCR-001` の「配信一覧が主コンテンツ」という情報階層を満たせない。

#### 修正単位

#54 で global App shell の責務を Header / Primary navigation / route host 中心へ戻し、画面固有レイアウトは各画面へ移す。

### 5.2 SCR-001 配信一覧

#### 仕様どおりの箇所

- URLは `/streams`
- Previewを行ってから登録する
- Preview成功後に登録操作が出るため、Previewを飛ばして登録する導線はない
- 登録成功後は `/streams/:streamId` へ直接遷移する (`FLW-002`)
- 登録済み配信選択後は `/streams/:streamId` へ直接遷移する (`FLW-001`)
- Preview loading / Registering / API error の状態を区別している

#### 修正が必要な箇所

- 一覧をMainの主コンテンツへ移す
- List / Tableへ変更する
- 新規追加を折りたたみ式の補助操作にする
- Hero / welcome / preview historyを削除する
- 仕様の7項目を表示する

#### API差分

現行 `StreamList` は `Stream[]` であり、metadataとして以下は取得できる。

- title
- channelTitle
- scheduledStartAt / actualStartAt
- durationMs
- lifecycleStatus

一方、以下は取得できない。

- Collection未実施を含む収集状態
- 収集済みChat message総件数

UI側でStreamごとに追加HTTP requestを行う構成にはせず、#56で一覧read modelを拡張する。

### 5.3 SCR-002 配信ワークスペース / Timeline

#### 仕様どおりの箇所

- canonical URLは `/streams/:streamId`
- Timeline専用routeは存在しない
- YouTube Playerが存在する
- Player時刻をReact stateへ取り込み、Chat message / Search resultからPlayerへseekできる
- playback timeに近いChat messageを `aria-current="time"` として識別している
- Chat search resultからseekするとPlayerとmessage current stateが同期する
- YouTube embed失敗時もChatを利用可能にし、YouTube外部リンクを提供する
- Collection未実施 / active / failed / no-data / retry可能状態を区別している

#### 修正が必要な箇所

- Workspace headerを最小情報へ整理する
- 詳細metadataをDialogへ移す
- Collection成功後の大型summaryを縮小する
- decorative eyebrowを除く
- Chat領域をglobal panelの表示状態から独立させる
- Chat activity棒グラフを追加する
- Chat list auto-follow / pause / resumeを追加する

#### API差分

現行Chat APIはmessage list / searchのcursor paginationのみである。

Timelineの棒グラフを長時間配信でも成立させるには、全Chat messageをブラウザへロードせず、5秒 / 10秒 / 30秒bucketの件数を取得できるAPI/read modelが必要である。

#58ではこのAPIとUIを1つのfeature sliceとして実装・検証する。

### 5.4 SCR-003 予約一覧

#### 仕様どおりの箇所

- URLは `/reservations`
- 予約作成成功後は `/reservations/:reservationId` へ直接遷移する (`FLW-006`)
- 予約行選択は独立詳細へ直接遷移する (`FLW-005`)
- Loading / Empty / Submitting / Error を区別している

#### 修正が必要な箇所

- Active reservationsを主コンテンツへする
- Create reservation UIを折りたたむ
- Active / Historyを分離する
- List / Tableに仕様の5項目を表示する
- 各行の長いnextAction説明を除く
- Hero / catch copy / decorative eyebrow / 恒常説明文を除く

#### API差分

現行Reservation modelは以下を一覧レスポンスで持つ。

- youtubeVideoId
- state
- scheduledStartAt
- nextCheckAt
- lastErrorCode / lastErrorMessage
- collectionError

配信タイトルは持たないが、SCR-003は「配信タイトルまたは識別情報」を要求しており、YouTube Video IDへのフォールバックを明示的に許可している。

したがって Reservations UI修正のためのAPI変更は必須ではない。

### 5.5 SCR-004 予約詳細

#### 仕様どおりの箇所

- 独立URL `/reservations/:reservationId` を使用している
- 一覧へ戻れる (`FLW-007`)
- Active状態をpollingで更新する
- 監視エラー / Collectionエラーを区別して表示する
- retryableかどうかをユーザー向けに表現する
- `canCancel` の場合のみCancel actionを出す
- `completed` かつ `streamId` がある場合に `/streams/:streamId` へ直接遷移する (`FLW-008`)

#### 修正が必要な箇所

- decorative eyebrowを除去する
- 状態名のHero的な見せ方を情報階層へ合わせる
- Terminal状態では不要な「次回確認」を残さない

## 6. 後続 Issue

### #54 Web App shellをUI仕様に合わせて再構成する

全画面の構造上の前提を修正する基盤Issue。

### #55 配信一覧をSCR-001 UI仕様に合わせて実装する

配信一覧・配信追加・Preview・不要UI除去を担当する。

### #56 配信一覧APIに収集状態とチャット件数を追加する

SCR-001の一覧read model不足を解消する。

### #57 配信ワークスペースの表示構造をSCR-002仕様に合わせる

Workspace header / Metadata Dialog / Collection presentation / Chat領域責務を修正する。

### #58 Timelineにチャット量グラフと時間bucket集計APIを実装する

Chat activityと必要な集計APIをend-to-endで実装する。

### #59 Chat message listの再生位置自動追従と手動スクロール制御を実装する

Player時間とChat list scrollの同期を完成させる。

### #60 Reservations UIをSCR-003 / SCR-004仕様に合わせて実装する

予約一覧 / 予約詳細を正本仕様へ合わせる。

## 7. 推奨実施順

依存関係を考慮した推奨順は以下とする。

```text
#54 App shell
├── #56 Stream list read model ──→ #55 SCR-001
├── #57 SCR-002 structure
│   ├──→ #58 Chat activity + aggregation API
│   └──→ #59 Chat auto-follow
└──→ #60 Reservations
```

#56は#54と並行実施可能である。

#58 / #59は#57完了後に実施すると、Workspace layoutの重複修正を避けやすい。

各Issueでは、その変更に対応するPlaywright E2Eを同じPRで更新し、`docs/ui/e2e-coverage.md` に記録した未検証項目を順次解消する。

## 8. 本Issueで実装修正しないもの

Issue #46 自体では次を行わない。

- React / CSSのUI修正
- API schema / repository / SQLの修正
- Playwright testの挙動変更

本書と後続Issueの起票のみを成果物とする。
