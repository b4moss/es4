---
状態: 方針確定
マイルストーン: v0.1.0（暫定 / Phase 0）
---

# Phase 0 — スキャフォールド

## 目的

リポジトリの骨格だけを置く。実装コード・仕様・テスト仕様は書かない。

## ざっくり範囲

- 必要なディレクトリツリーを **`.gitkeep` のみ**で追加する
- 正確なパッケージ分割は [open-questions P8](../open-questions.md) 未決のため、以下は **暫定プレースホルダ**とする（公開面は State API のみ、[Phase 1 決定事項](../v0.2.0/minimal-core.md#決定事項)）
- 多言語ポートと Docker を見据え、言語別コードは `packages/<lang>/` 配下、コンテナ関連は `docker/` に置く

暫定ツリー（空ディレクトリ + `.gitkeep`）:

```
es4/
├── docs/                         # OKF 文書（本マイルストーンでは触らない）
├── packages/
│   ├── go/
│   │   ├── cmd/
│   │   │   └── es4-server/       # Server エントリ（実装は後続 Phase）
│   │   ├── internal/
│   │   │   ├── state/
│   │   │   ├── snapshot/
│   │   │   ├── recovery/
│   │   │   └── config/
│   │   └── pkg/                  # 公開 API プレースホルダ
│   └── node/
│       └── src/                  # Node.js / TypeScript ポート用（Go 版の後）
├── docker/                       # コンテナ関連（Dockerfile は Server 実装時に追加）
└── scripts/                      # 補助スクリプト用（任意）
```

配置の意図:

- **Server** は `packages/go/cmd/es4-server/` に置く
- **ライブラリ**（State / Snapshot / Recovery / config および公開面）は `packages/<lang>/` 配下
- **Docker** は言語ツリーの外（`docker/`）。本マイルストーンでは `.gitkeep` のみ。`Dockerfile` は Go Server のビルドが始まるときに追加する（compose も後続）

既存の `docs/specs/`・`docs/tests/` はそのまま（空プレースホルダでよい）。  
ルート直下の `internal/` は置かない（旧プレースホルダは廃止）。

## 範囲外

- State / Snapshot / Recovery の振る舞い実装
- `go.mod` や Go / TypeScript のソース追加
- `Dockerfile` / compose の実装
- `docs/specs/`・`docs/tests/` への仕様記入

## メモ

- SemVer `v0.1.0` は暫定割当。正式版名が決まり次第フォルダと roadmap を更新する。
- ツリー名は P8 確定後に改名してよい。本マイルストーンは「空ディレクトリが存在する」ことだけを成果とする。
- 関連: [roadmap](../../roadmap.md) · [pillar](../../README.md) · [open-questions](../open-questions.md)（P8）

----

以上
