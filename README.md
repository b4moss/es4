# Es4: An Server-side State Storage

## 開発の同期

- Redis/Valkeyを導入するほどではないが、しかし一定の機能を備えたサーバーサイドステートを持ちたい
- Redisの永続化層のように、インメモリではあるが揮発しない機能が欲しい
- 組み込みと独立、ケースバイケースで実装を選べるようにしたい
- 各層で様々なサービスを選択できるようにしたい(Adapter化)

## 言語

- Go(初期実装)
- Node.js(TypeScript: Go版の実装後)

## 技術設計

- インメモリのステートストアと、そのAPI口
- ステートをリカバリするための永続化層
- 上記2層は、アダプター化され、複数のドライバーを設定できる

## アーキテクチャのイメージ(構想段階)

```mermaid
flowchart TB
    APP["Application"]

    subgraph ES4["Es4"]
        API["State API"]

        subgraph STATE["State Store"]
            MEMJSON["Memory JSON"]
            MEMSQL["Memory SQLite"]
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

    App->>Es4: SET / GET / DELETE
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

## 暫定ロードマップ

### Phase 1 — 最小構成

* インメモリ JSON State
* ファイルベースの Recovery Storage
* Snapshot / Restore
* スナップショット間隔の設定
* 明示的なスナップショット取得
* 起動時の復元
* 永続化なしの Memory-only モード

目標:
State → Snapshot → Recovery というEs4の基本ライフサイクルを確立する。

⸻

### Phase 2 — SQLite State

* インメモリ SQLite State
* SQLiteを利用したスナップショット
* ファイルベースの Recovery Storage
* トランザクション対応
* 並行アクセスへの対応

目標:
構造化されたStateと、より高頻度なState操作に対応する。

⸻

### Phase 3 — 外部 Recovery Storage

* オブジェクトストレージAdapter
    * S3互換ストレージ
* libSQL Adapter
* Litestream連携 / Adapter
* スナップショットの世代管理
* スナップショットの保持ポリシー

目標:
Recovery Storageをローカルファイルシステムから切り離し、外部ストレージへ拡張する。

⸻

### Phase 4 — Es4 Server

* HTTP API
* Dockerイメージ
* 環境変数による設定
* Health / Readiness Endpoint
* 外部 Recovery Storage
* Cloud Runへの対応

目標:
複数のアプリケーション・インスタンスから共有できるState Storeとして利用可能にする。

⸻

### Phase 5 — State Backendの拡張

* その他のインメモリ実装
* その他の組み込みデータベース
* Redis / Valkey Adapter（具体的なユースケースが生じた場合）
* State Backendごとの最適化

目標:
コアアーキテクチャを特定のState Backendに依存させず、柔軟性を拡張する。
