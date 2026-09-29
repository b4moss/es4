# Es4: A Server-side State Storage

プロダクトの意味的な pillar 正本（目的・スコープ・技術方針のハブ）。  
OKF の版索引は [`index.md`](./index.md)（`okf_version` のみ）。本文はここに書く。

実装前の構想を含む。現行の振る舞い仕様は [`specs/`](./specs/)（Options / State / Tx / Snapshot / Recovery / Es4 Server）。これからやる内容は [`roadmap.md`](./roadmap.md) と [`plans/`](./plans/) を参照。

## 目的・動機（開発の同期）

- Redis/Valkeyを導入するほどではないが、しかし一定の機能を備えたサーバーサイドステートを持ちたい
- Redisの永続化層のように、インメモリではあるが揮発しない機能が欲しい
- 組み込みと独立、ケースバイケースで実装を選べるようにしたい
- 各層で様々なサービスを選択できるようにしたい(Adapter化)

## 言語

- Go（初期実装）
- Node.js（TypeScript: Go版の実装後）

## 技術設計

- インメモリのステートストアと、そのAPI口
- ステートをリカバリするための永続化層
- 上記2層は、アダプター化され、複数のドライバーを設定できる

## アーキテクチャのイメージ（構想段階）

実装済みの現行仕様ではなく、PO が決めた構想図である。

```mermaid
flowchart TB
    APP["Application"]

    subgraph ES4["Es4"]
        API["State API"]

        subgraph STATE["State Store"]
            MEMJSON["Memory JSON"]
            FILESQL["File SQLite"]
            OTHER["Other State Backend"]
        end

        SNAP["Snapshot Manager"]

        API --> STATE
        STATE --> SNAP
    end

    APP --> API

    subgraph RECOVERY["Recovery Storage"]
        FILE["File"]
        OBJECT["Object Storage\nS3 / GCS"]
        LIBSQL["libSQL"]
        OTHER_REC["Other Adapter"]
    end

    SNAP --> FILE
    SNAP --> OBJECT
    SNAP --> LIBSQL
    SNAP --> OTHER_REC
```

## ライフサイクル

```mermaid
sequenceDiagram
    participant App
    participant Es4
    participant State as State Store
    participant Recovery as Recovery Storage

    App->>Es4: SET / GET / DELETE / EXISTS / CLEAR
    Es4->>State: Update / Read
    State-->>Es4: Current State
    Es4-->>App: Result

    Note over Es4,Recovery: Snapshot interval

    Es4->>State: Create Snapshot
    State-->>Es4: Snapshot
    Es4->>Recovery: Save Snapshot

    Note over App,Recovery: Process / Instance failure

    Es4->>Recovery: Load Snapshot
    Recovery-->>Es4: Snapshot
    Es4->>State: Restore
    State-->>Es4: Ready
```

```mermaid
flowchart LR
    STATE["Current State\n揮発してよい"]
    SNAP["Snapshot"]
    REC["Recovery Point\n復旧のために残す"]

    STATE -->|periodic / explicit| SNAP
    SNAP --> REC
    REC -->|restart / failure| STATE
```

## 利用想定

- ライブラリとしての組み込み
- 単独アプリケーション

の両利用を想定

```mermaid
flowchart LR
    subgraph EMBEDDED["Embedded"]
        APP1["Application"]
        ES41["Es4 Library"]
        MEM1["In-process State"]
        REC1["Recovery Storage"]

        APP1 --> ES41
        ES41 --> MEM1
        ES41 --> REC1
    end

    subgraph SERVER["Server"]
        APP2["Application"]
        ES42["Es4 Server\nDocker"]
        MEM2["Server-local State"]
        REC2["Recovery Storage"]

        APP2 -->|HTTP| ES42
        ES42 --> MEM2
        ES42 --> REC2
    end
```

## 索引

- [roadmap](./roadmap.md) — マイルストーン（Phase 0–5 / 暫定 SemVer `v0.1.0`–`v0.6.0`）
- [plans](./plans/) — これからやる内容
- [Phase 0 決定事項](./plans/v0.1.0/scaffold.md#決定事項) — スキャフォールド完了・module path・最初の Git タグ方針
- [Phase 1 仕様](./specs/state/) — State / [Snapshot](./specs/snapshot/) / [Recovery](./specs/recovery/) / [Options](./specs/options/)（`v0.2.0` 実装完了・plans から移動済み）
- [Phase 2 仕様](./specs/state/) — SQLite State / [Tx](./specs/tx/) / [Snapshot](./specs/snapshot/) / [Options](./specs/options/)（`v0.3.0` 実装完了・plans から移動済み）
- [Phase 3 Recovery / Options](./specs/recovery/) — libSQL / Object・世代 TTL・GCS＝S3 互換
- [Phase 4 仕様](./specs/es4-server/) — HTTP Es4 Server・Docker・Health（末尾 `/` なし。`v0.5.0` 実装完了・plans から移動済み）
- [Phase 5 決定事項](./plans/v0.6.0/state-backend-extension.md#決定事項) — その他／Redis・Valkey Unscheduled・最適化境界
- [open-questions](./plans/open-questions.md) — 未決プロダクト論点（**残件なし**）
- [wishlist](./wishlist.md) — PO メモ（未整理）
- [specs](./specs/) — 現行仕様
- [tests](./tests/) — テスト仕様
- [憲章](./charter/) — 開発ルール

----

以上
