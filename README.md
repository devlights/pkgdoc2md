# pkgdoc2md

`pkg.go.dev` の特定パッケージページをMarkdownに変換するCLIツールです。  
LLMへのアップロード用途を想定して、ドキュメント本文のみを抽出します。

HTMLからMarkdownへの変換には [html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown/tree/v1.6.0) を利用しています。

最新版の [html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown/tree/main) では GFM スタイルに変換するプラグインが無くなっているので、v1.6.0 を使ってます。

## インストール

```bash
go install github.com/devlights/pkgdoc2md@latest
```

## 使い方

```bash
# stdoutに出力
./pkgdoc2md -pkg net/http

# ファイルに出力
./pkgdoc2md -pkg net/http -output net_http.md

# タイムアウトを60秒に変更
./pkgdoc2md -pkg encoding/json -output encoding_json.md -timeout 60

# デバッグログを有効化
./pkgdoc2md -pkg context -debug
```

## フラグ

| フラグ      | デフォルト | 説明                                      |
|-------------|------------|-------------------------------------------|
| `-pkg`      | (必須)     | 変換対象のパッケージパス (例: `net/http`) |
| `-output`   | stdout     | 出力先ファイルパス                        |
| `-timeout`  | 30         | HTTPリクエストのタイムアウト秒数          |
| `-debug`    | false      | デバッグログを有効化                      |

