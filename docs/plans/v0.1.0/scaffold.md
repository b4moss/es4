---
状態: スキャフォールド完了
マイルストーン: v0.1.0（暫定 / Phase 0）
---

# Phase 0 — スキャフォールド

## 目的

リポジトリの骨格だけを置く。実装コード・仕様・テスト仕様は書かない。

## ざっくり範囲

- 必要なディレクトリツリーを **`.gitkeep` のみ**で追加する
- Go module path・配置レイアウトは下記の決定どおり（公開面は State API のみ、[Phase 1 決定事項](../v0.2.0/minimal-core.md#決定事項)）
- 多言語ポートと Docker を見据え、言語別コードは `packages/<lang>/` 配下、コンテナ関連は `docker/` に置く

ツリー（空ディレクトリ + `.gitkeep`）:

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

- **Go module path:** `github.com/b4moss/es4/packages/go`（`go.mod` 本体は後続 Phase で追加）
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
- Git タグの作成（下記ポリシー。タグ打ちは本マイルストーンの PR 外）

## 決定事項

- **レイアウト:** 上記ツリーを Phase 0 の正とする（スキャフォールド完了）
- **Go module path:** `github.com/b4moss/es4/packages/go`
- **最初の Git タグ:** スキャフォールド完了を条件に `v0.1.0` を打つ。タグ作成はコーディネータが main 上で行う（本 PR ではタグを作らない）

## メモ

- SemVer `v0.1.0` はスキャフォールド完了時点の最初の Git タグ候補。後続 Phase の正式版名が決まり次第、フォルダと roadmap を更新する。
- 本マイルストーンの成果は「空ディレクトリが存在する」こと。実装コードは載せない。
- 関連: [roadmap](../../roadmap.md) · [pillar](../../README.md) · [versioning-rule](../../charter/versioning-rule.md) · [Phase 1 決定事項](../v0.2.0/minimal-core.md#決定事項)

----

以上
