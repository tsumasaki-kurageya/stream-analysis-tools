# SCR-002 配信ワークスペース UI仕様

この文書は、配信の収集・確認・分析を行う配信ワークスペースの UI 仕様の正本である。

横断ルールは `docs/ui/principles.md`、画面遷移は `docs/ui/navigation.md` を参照する。

## 1. Purpose

配信ワークスペースの主目的は、**1つの配信を時間軸に沿って確認・分析するための起点を提供すること**である。

単なる配信詳細画面ではなく、動画再生、チャット量の把握、チャット内容の確認、収集状態の確認、および将来の分析ビューへの入口を統合する Workspace として扱う。

優先順位は次のとおり。

1. 動画の現在再生位置を基準に配信内容を確認する
2. チャット量の変化から注目すべき時間帯を探索する
3. 再生位置に対応するチャットメッセージを確認する
4. 必要に応じてチャットを検索する
5. 収集未完了・失敗時に収集処理を実行または再試行する
6. 配信メタデータの詳細を必要時に確認する
7. 将来の Timeline / Location / PC view へ移動する

## 2. Entry

主な Entry は以下とする。

- `SCR-001` 配信一覧から対象配信を選択する (`FLW-001`)
- 配信登録完了後に登録対象の Workspace へ遷移する (`FLW-002`)
- Workspace の URL への direct access / reload
- 将来の Workspace 子ビューから Workspace 共通領域を維持して遷移する

## 3. Workspace の基本構造

Workspace は、共通領域と分析ビュー領域を分けて扱う。

現時点の基本構造は以下とする。

```text
Workspace
├── Workspace header
│   ├── Stream title
│   ├── Channel
│   ├── Stream status
│   ├── Collection status
│   └── Metadata info action
│
├── Collection attention area [必要時のみ]
│
├── YouTube Player
│
├── Chat activity
│   ├── Aggregation interval selector
│   │   ├── 5秒
│   │   ├── 10秒 [default]
│   │   └── 30秒
│   └── Chat count bar chart
│
├── Chat tools
│   └── Chat search
│
└── Chat message list
```

将来の Timeline / Location / PC view は、同一ページへ無制限に縦積みせず、URL を持つ Workspace の子ビューとして扱う。詳細な URL と共通領域の境界は #44 で定義する。

## 4. Workspace header

Workspace header は、分析対象を識別するための最小限の情報に限定する。

### 常時表示する情報

- 配信タイトル
- チャンネル
- 配信状態
- 収集状態
- 配信一覧へ戻るための導線
- Metadata detail を開くための info action

### 常時表示しない情報

以下は分析領域を圧迫するため、header に大きく常時表示しない。

- 配信日時の詳細
- 配信時間の詳細
- YouTube video ID
- Canonical URL
- その他の技術的 metadata

これらは Metadata detail で確認する。

## 5. Metadata detail

配信メタデータの詳細は専用画面へ遷移せず、**info icon 等の明示的な操作から Dialog として表示する**。

### 表示候補

最低限、取得可能な以下の情報を表示する。

- タイトル
- チャンネル
- 配信日時
- 配信時間
- 配信状態
- YouTube video ID
- YouTube の canonical URL

必要に応じて、ユーザー判断に有用な追加 metadata を表示してよい。

### 原則

- Dialog は metadata 詳細確認のためにのみ使用する
- Workspace の主要操作を Dialog 内へ移さない
- Dialog を開かなくても分析作業を継続できる
- 技術内部情報を無制限に露出しない

## 6. YouTube Player

YouTube Player は Workspace の共通要素として**常時表示する**。

Player の現在再生位置は、Chat activity と Chat message list が共有する基準時刻とする。

### 必須動作

- Player の再生位置を UI 側で継続的に把握できる
- Chat activity の棒を選択した場合、その時間帯へ seek する
- Chat message を時間指定で選択した場合、そのメッセージ時刻へ seek できる
- 埋め込み再生不可の場合でも、チャット分析データは利用可能な状態を維持する
- 埋め込み不可時は YouTube で開く代替導線を提供する

## 7. Chat activity

Chat activity は、配信中のチャット量の変化を時間軸で俯瞰するための共通分析領域とする。

### グラフ形式

- 棒グラフ
- 横軸: 配信開始からの経過時間
- 縦軸: 集計区間内のチャットメッセージ件数

### 集計単位

ユーザーは以下から選択できる。

- 5秒
- 10秒
- 30秒

**初期値は 10秒** とする。

集計区間は配信開始を基準として連続した bucket とする。

例: 10秒の場合

```text
00:00-00:09
00:10-00:19
00:20-00:29
...
```

### 棒グラフの操作

各棒は、その集計区間を表す操作対象とする。

棒を選択した場合:

1. 対応する集計区間の開始時刻を seek target とする
2. YouTube Player をその再生位置へ移動する
3. Chat message list も新しい再生位置へ同期する
4. 現在再生位置を含む棒を active state として視覚的に識別可能にする

### 集計単位変更時

- Player の現在再生位置は維持する
- 新しい集計単位でグラフを再描画する
- 現在再生位置を含む新しい bucket を active state とする

### データがない場合

収集済みチャットが0件の場合、0件であることを Empty state として明示する。

収集未完了の場合は、チャットが0件であることと収集未完了を混同しない。

## 8. Chat message list

Chat message list は Workspace の共通分析領域とし、Player の再生時間と同期する。

### 基本動作

- メッセージを時系列で表示する
- 各メッセージには少なくとも再生位置と本文を識別できる情報を持たせる
- 現在再生位置に対応するメッセージを active / current として識別可能にする
- Player の再生位置が進むと、対応するチャット位置へ自動スクロールする
- メッセージを選択して、その時刻へ Player を seek できる

### 自動追従

初期状態では **Player の再生位置へ自動追従する**。

ユーザーが Chat message list を手動スクロールした場合:

1. 自動追従を一時停止する
2. Player の再生自体は停止しない
3. ユーザーが過去・未来のチャットを自由に閲覧できる
4. `再生位置に戻る` 等の明示的な操作を表示する
5. その操作を実行すると、現在の Player 再生位置へスクロールし、自動追従を再開する

自動追従停止中も、Player の現在再生位置そのものは保持・更新する。

### 自動スクロールの原則

- active message が表示領域から外れるたびに不必要な大移動を繰り返さない
- ユーザーの手動スクロール操作を自動追従が即座に奪わない
- 自動追従中か停止中かを必要に応じて識別可能にする

## 9. Chat search

Chat Search は Chat message list と同じ Workspace 共通領域で利用できる。

### 原則

- 検索結果から対象メッセージ時刻へ Player を seek できる
- 検索によって通常の Chat message list を恒久的に置き換えない
- 検索結果から seek した場合、Player / Chat activity / Chat message list の基準時刻を同じ時刻へ同期する
- 検索結果0件と検索エラーを区別する

検索 UI の詳細なレイアウトは、主分析領域を圧迫しない範囲で決定してよい。

## 10. Collection

Collection は分析そのものではなく、分析データを準備するための前処理として扱う。

### 収集未実施・処理中・失敗時

分析に必要なチャットデータが利用できないため、Collection 状態と必要な操作を明確に表示する。

必要に応じて以下を表示する。

- 未収集
- queued
- running
- failed
- no data
- retryable / non-retryable
- Start / Retry action
- Processing progress / counts

### 収集成功後

Collection が成功し分析可能になった後は、Collection UI を Workspace の主コンテンツとして大きく表示し続けない。

- Header 等で収集済みであることを短く確認できる
- 詳細な processed / skipped / attempt 等は必要に応じた補助情報とする
- Player / Chat activity / Chat message list の表示領域を優先する

### 収集失敗時

- retryable な場合のみ Retry action を提供する
- failure reason はユーザーが判断可能な表現にする
- collector 内部実装・command・raw stderr 等は露出しない

## 11. 時刻同期モデル

Workspace では **現在再生位置を1つの共通状態として扱う**。

以下は同じ playback time を参照する。

- YouTube Player
- Chat activity の active bucket
- Chat message list の current message
- Chat search result からの seek

### 時刻変更の入力元

再生位置は以下から変更される。

- Player 自身の再生
- Player の seek 操作
- Chat activity の棒選択
- Chat message の選択
- Chat search result の選択

どの入力元から変更されても、他の時刻連動 UI は同じ新しい再生位置へ同期する。

## 12. 将来の分析ビュー

Timeline / Location / PC view は、Workspace にすべて縦積みしない。

**URL を持つ Workspace 子ビュー**として設計する。

候補 URL は #44 で確定する。

```text
/streams/:streamId/timeline
/streams/:streamId/locations
/streams/:streamId/pcs
```

#42 では以下のみを確定する。

- Workspace は分析の共通コンテキストを保持する
- 将来分析ビューは独立した URL を持つ
- 子ビュー切替のために Workspace 全体を再設計しなくて済む構造にする
- Player / Chat activity / Chat message list のどこまでを各子ビューでも共通表示するかは #44 で決定する

## 13. Visible elements

分析可能な通常状態では最低限以下を表示する。

- Workspace header
- 配信タイトル
- チャンネル
- 配信状態
- 収集状態
- Metadata info action
- YouTube Player
- Chat activity
- 5秒 / 10秒 / 30秒の集計単位 selector
- Chat Search
- Chat message list
- 現在再生位置と対応する active state

必要な状態に限り以下を表示する。

- Collection Start / Retry
- Collection processing state
- Collection failure
- `再生位置に戻る` 等の follow resume action
- Metadata Dialog
- Player unavailable fallback

## 14. Forbidden elements

以下を恒常的に表示しない。

- 大型 Stream detail Hero
- Metadata の大型常時表示領域
- YouTube video ID 等の技術 metadata の常時露出
- Collection 成功後の大型 Collection panel
- 分析機能の長い説明文
- Marketing copy
- decorative eyebrow text
- Timeline / Location / PC view の縦積み
- 仕様にない Modal / Card / Help panel

また、Player / graph / message list を互いに独立した時刻状態として実装しない。

## 15. States

### Workspace loading

配信情報の取得中であることを明示する。

### Workspace ready / chat available

Player、Chat activity、Chat Search、Chat message list が利用可能な通常状態。

### Chat collection not started

チャット分析データがまだ存在しないことと、収集開始操作を提示する。

### Collection queued / running

処理中であることを明示し、チャットデータが未確定であることを表現する。

### Collection succeeded

収集成功状態を短く表示し、分析領域を主役にする。

### Collection no data

収集は完了したが利用可能なチャットがない状態として Empty と区別する。

### Collection failed

失敗理由と、可能であれば Retry action を表示する。

### Chat empty

収集成功済みだが表示対象チャットが0件である状態。

### Chat search empty

検索結果のみ0件である状態。Chat全体が空である意味にはしない。

### Player unavailable

埋め込み再生不能を表示し、YouTube で開く代替導線を提供する。収集済みチャットは引き続き利用可能とする。

### Follow paused

ユーザーの手動スクロールにより Chat message list の自動追従が停止している状態。再生位置へ戻る操作を提供する。

### Error

配信取得、Collection、Chat、Search 等のエラーを発生元と対応付けて表示する。

## 16. Data / API requirements

本仕様を実現するため、UI は少なくとも以下のデータを取得できる必要がある。

- Stream metadata
- Collection status
- 時刻付き Chat messages
- 配信全体の Chat count aggregation
- 5秒 / 10秒 / 30秒単位の Chat count または集計可能な入力データ

長時間配信を前提とし、棒グラフ表示のために全 Chat message を UI へ一括取得してクライアントだけで集計することを必須前提にしない。

現行 API に適切な集計 endpoint / read model がない場合は、#46 の差分整理後に後続 Issue として扱う。

## 17. Acceptance Criteria

- [ ] Workspace の主目的が配信分析の起点として表現されている
- [ ] YouTube Player が Workspace 共通要素として常時表示される
- [ ] Metadata 詳細が info action から Dialog で確認できる
- [ ] Metadata の詳細が分析領域を常時圧迫しない
- [ ] Collection 未完了・処理中・失敗時は必要な状態・操作が目立つ
- [ ] Collection 成功後は Collection UI が縮小され、分析領域が主役になる
- [ ] Chat activity が時間 × チャット数の棒グラフとして表示される
- [ ] Chat activity の集計単位を 5秒 / 10秒 / 30秒から選択できる
- [ ] Chat activity の初期集計単位が 10秒である
- [ ] 棒グラフ選択で対応する時間へ Player が seek する
- [ ] 現在再生位置を含む棒が active state になる
- [ ] Chat message list が Player 再生位置へ自動追従する
- [ ] Chat message list の手動スクロールで自動追従が一時停止する
- [ ] 自動追従停止時に現在再生位置へ戻り、追従を再開できる
- [ ] Chat message 選択で対応時間へ Player を seek できる
- [ ] Chat search result 選択で Player / graph / list が同じ時刻へ同期する
- [ ] Player 埋め込み不可でも Chat 分析を継続できる
- [ ] Timeline / Location / PC view を同一画面へ縦積みしない
- [ ] 将来分析ビューは URL を持つ Workspace 子ビューとして拡張できる
- [ ] 仕様にない説明文・大型 Hero・常時 metadata 詳細を追加していない

## 18. Out of scope

- 現行 Workspace の実装修正
- Chat activity API / aggregation endpoint の実装
- Timeline / Location / PC view 自体の実装
- Timeline / Location / PC view の具体的 URL 確定
- 子ビューごとの Player / Chat 共通表示範囲の確定
- グラフの色・線幅・Animation 等のビジュアルデザイン詳細

これらは #44、#46、および後続の実装 Issue で扱う。
