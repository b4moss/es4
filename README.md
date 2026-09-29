# Es4: An Server-side State Storage

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
