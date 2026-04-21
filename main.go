package main

// pkg.go.dev の特定パッケージページをMarkdown形式に変換して出力するCLIツール。
// LLMへのアップロード用途を想定しているため、ナビゲーションやフッターなどの
// 不要なHTML要素を除去し、ドキュメント本文のみを抽出する。

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
)

const (
	pkgBaseURL = "https://pkg.go.dev"
)

type Args struct {
	pkg     string
	timeout int
	output  string
	debug   bool
}

var (
	args Args

	appLog = log.New(os.Stdout, "", 0)
	errLog = log.New(os.Stderr, "[ERROR] ", log.Lmicroseconds)
	dbgLog = log.New(os.Stdout, "[DEBUG] ", log.Lmicroseconds)
)

func init() {
	flag.StringVar(&args.pkg, "pkg", "", "変換対象のパッケージパス (例: net/http, encoding/json) [必須]")
	flag.IntVar(&args.timeout, "timeout", 30, "HTTPリクエストのタイムアウト秒数")
	flag.StringVar(&args.output, "output", "", "出力先ファイルパス (省略時はstdout)")
	flag.BoolVar(&args.debug, "debug", false, "デバッグログを有効にする")
}

func main() {
	flag.Parse()

	if !args.debug {
		dbgLog.SetOutput(io.Discard)
	}

	if args.pkg == "" {
		flag.Usage()
		errLog.Println("パッケージパスは必須: -pkg フラグを指定してください")
		return
	}

	var (
		ctx = context.Background()
		err error
	)
	if err = run(ctx); err != nil {
		errLog.Panic(err)
	}
}

func run(pCtx context.Context) error {
	var (
		ctx, cxl = context.WithCancel(pCtx)
		err      error
	)
	defer cxl()

	var (
		pkgUrl   = pkgBaseURL + "/" + strings.TrimLeft(args.pkg, "/")
		timeout  = time.Duration(args.timeout) * time.Second
		html     string
		body     string
		markdown string
	)
	dbgLog.Printf("取得開始: %s", pkgUrl)
	{
		if html, err = fetch(ctx, pkgUrl, timeout); err != nil {
			return fmt.Errorf("HTMLの取得に失敗しました: %w", err)
		}
	}
	dbgLog.Printf("HTML取得完了: %d bytes", len(html))
	{
		if body, err = extract(html); err != nil {
			dbgLog.Printf("本文抽出失敗、HTML全体を使用します: %v", err)
			body = html
		}
	}
	dbgLog.Printf("本文抽出完了: %d bytes", len(body))
	{
		if markdown, err = convert(body, pkgUrl); err != nil {
			return fmt.Errorf("Markdownへの変換に失敗しました: %w", err)
		}
	}
	dbgLog.Printf("Markdown変換完了: %d bytes", len(markdown))
	{
		if err = write(markdown, args.output); err != nil {
			return fmt.Errorf("出力の書き込みに失敗しました: %w", err)
		}

		appLog.Printf("出力: %s", args.output)
	}
	dbgLog.Printf("完了: Markdown %d bytes 出力", len(markdown))

	return nil
}

// fetch は指定URLのHTMLを取得して文字列として返す。
func fetch(ctx context.Context, url string, timeout time.Duration) (string, error) {
	var (
		client = &http.Client{
			Timeout: timeout,
		}
		req *http.Request
		err error
	)
	if req, err = http.NewRequestWithContext(ctx, http.MethodGet, url, nil); err != nil {
		return "", fmt.Errorf("リクエスト生成に失敗しました: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; pkgdoc2md/1.0)") // 一般的なブラウザを模倣
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	var (
		resp *http.Response
	)
	if resp, err = client.Do(req); err != nil {
		return "", fmt.Errorf("HTTPリクエストに失敗しました: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTPステータスエラー: %d %s", resp.StatusCode, resp.Status)
	}

	const (
		maxBodySize = 10 * 1024 * 1024
	)
	var (
		r    = io.LimitReader(resp.Body, maxBodySize)
		body []byte
	)
	if body, err = io.ReadAll(r); err != nil {
		return "", fmt.Errorf("レスポンスボディの読み取りに失敗しました: %w", err)
	}

	return string(body), nil
}

// extract はpkg.go.devのHTMLから、ドキュメント本文部分のHTMLを抽出する。
//
// 抽出対象の要素:
//   - <main> タグの内容 (主要コンテンツ領域)
func extract(htmlContent string) (string, error) {
	// <main>タグの内容を単純な文字列探索で抽出する。
	// ネストしたmainタグがある場合は誤動作する可能性があるが
	// pkg.go.devのページではmainタグは1つだけなので実用上問題ない。
	const (
		openTag  = "<main" // > が無いのは意図的。属性が存在する場合を考慮。
		closeTag = "</main>"
	)
	var (
		start = strings.Index(htmlContent, openTag)
		end   = strings.LastIndex(htmlContent, closeTag)
	)
	if start == -1 {
		return "", fmt.Errorf("<main>タグが見つかりませんでした")
	}

	if end == -1 {
		return "", fmt.Errorf("</main>タグが見つかりませんでした")
	}

	// </main>タグ自体も含める
	end += len(closeTag)

	return htmlContent[start:end], nil
}

// convert はHTML文字列をMarkdown形式に変換する。
func convert(htmlContent string, sourceURL string) (string, error) {
	var (
		converter = md.NewConverter(
			"pkg.go.dev",
			true,
			nil,
		)
	)
	converter.Use(plugin.GitHubFlavored()) // GitHub Flavored Markdown (GFM) を有効化

	var (
		markdown string
		err      error
	)
	if markdown, err = converter.ConvertString(htmlContent); err != nil {
		return "", fmt.Errorf("変換処理に失敗しました: %w", err)

	}

	// ヘッダーコメントを先頭に付与 (LLMがコンテキストを把握しやすくなるため)
	header := fmt.Sprintf("<!-- source: %s -->\n\n", sourceURL)

	return header + markdown, nil
}

// write はMarkdown文字列を指定の出力先に書き込む。
// outputPath が空文字の場合は標準出力に書き込む。
func write(content string, outputPath string) error {
	if outputPath == "" {
		_, err := fmt.Fprint(os.Stdout, content)
		return err
	}

	var (
		file *os.File
		err  error
	)
	if file, err = os.Create(outputPath); err != nil {
		return fmt.Errorf("ファイルの作成に失敗しました %q: %w", outputPath, err)
	}
	defer file.Close()

	if _, err = fmt.Fprint(file, content); err != nil {
		return fmt.Errorf("ファイルへの書き込みに失敗しました %q: %w", outputPath, err)
	}

	return nil
}
