[English](README.md) | [日本語](README.ja.md)

# GoReadManga
Find, read, maybe.

[![Go Report Card](https://goreportcard.com/badge/github.com/stl3/GoReadManga)](https://goreportcard.com/report/github.com/stl3/GoReadManga)
### 機能 ✨

- 🚀 **便利で高速**: 簡単にマンガを素早く取得し、検索できます。
- 🔄 **中断した場所から再開**: 読書セッションを簡単に続けられます。
- 🕵️‍♂️ **履歴の閲覧**: ネイティブにインストールされた `fzf` を使用して以前に閲覧した資料にアクセスするか、インストールされていない場合は組み込みの `fzf` 検索を利用します。
- 📁 **PDFストレージ**: 生成されたPDFは、OSの一時ディレクトリに保存されます（Windows、Android、Linux、Darwinに対応）。
- 🖼️ **画像処理**: 効率的な画像のエンコード/デコードのために `jpegli` または標準JPEGライブラリを選択できます。
- 📄 **縦画像の分割**: 高い縦画像を隙間なく複数ページに分割します。
- 🌐 **横画像の分割**: 幅広の横画像を複数ページに分割します（画像を縦に最大化）。
- 📊 **視聴統計**: 読書習慣に関する基本的な統計情報を取得します。
- 🔄 **サーバー切り替え**: 異なるコンテンツサーバー間で簡単に切り替えられます。
- 🧹 **キャッシュ管理**: キャッシュを簡単にクリアできます（すぐに大きくなることがあります！）。
- 🔧 **エラーハンドリング**: ネットワークの切断や障害によって発生した履歴JSONファイル内の壊れたエントリを削除します。
- 🗂️ **包括的な履歴追跡**: すべての履歴ファイルから統計情報を読み取ります（メインの履歴JSONファイルが5MBに達するとバックアップが作成されます）。
- 🌐 **プロキシサポート**: `-ph`, `--proxy-host` オプションを使用してSOCKS5プロキシを利用します [`server:port`]
  
### 🔍 今後の機能:
- 📂 **カスタム出力ディレクトリ**: `-o`, `--output-dir` オプションを使用して出力ディレクトリを指定します。
- 🎨 **カスタマイズ可能なPDF背景**: PDF内の空白の色を変更できます（デフォルト: 黒）。
- 🎯 **タイトルベースの推薦**: 提供されたタイトルに基づくレコメンダー。
- 🎲 **ランダム化オプション**: ランダム化またはジャンルに基づいてランダム化します。

💡 **今後の予定**: 追加機能の開発にご期待ください！

![image](https://github.com/user-attachments/assets/0e1792f4-dbc6-4bf0-8217-bb27a97c4cfc)


### Commands in program
![image](https://github.com/user-attachments/assets/1cb7862b-1800-4f92-8c0a-f74be3f9df11)






| コマンド | 説明 |
|---|---|
| `N` | 次の章 |
| `P` | 前の章 |
| `S` | 章を選択 |
| `R` | 現在の章を再オープン |
| `A` | 別のマンガを検索 |
| `BH` | 履歴を閲覧し、選択して読む |
| `ST` | 統計を見る |
| `OD` | PDFディレクトリを開く |
| `CS` | コンテンツサーバー1と2を切り替え |
| `D` | 画像デコード方式を切り替え [jpegli/通常] |
| `M` | jpegliエンコーディングモードを切り替え [jpegli/通常] |
| `WS` | ページよりも広い画像の分割を切り替え |
| `C` | キャッシュをクリア |
| `Q` | 終了 |

### コマンドライン引数
![image](https://github.com/user-attachments/assets/d6cf98b7-a4f9-4762-975f-b6a7054348d0)



**使用法:**

  GoReadManga [オプション]

**オプション:**

| オプション                     | 説明                                                      |
|-------------------------------|----------------------------------------------------------|
| `-h`, `--help`               | このヘルプページを表示                                   |
| `-v`, `--version`            | バージョン番号を表示                                     |
| `-jp`, `--jpegli`            | jpegliを使用してJPEGを再エンコード                      |
| `-q`, `--quality`            | jpegliエンコーディングに使用する品質を設定（デフォルト: 85） |
| `-ws`, `--wide-split`        | 幅が広すぎる画像を分割し、縦に最大化                    |
| `-ph`, `--proxy-host`        | SOCKS5プロキシサポート [サーバー:ポート]               |
| `-H`, `--history`            | 履歴における最後に閲覧したマンガのエントリを表示        |
| `-bh`, `--browse-history`    | 履歴ファイルを閲覧し、選択して読む                      |
| `-st`, `--stats`             | 履歴統計を表示                                          |
| `-r`, `--resume`             | 最後のセッションから続行                                 |
| `-od`, `--opendir`           | PDFディレクトリを開く                                   |
| `-c`, `--cache-size`         | キャッシュサイズを表示 (C:\Users\Administrator\AppData\Local\Temp\.cache\goreadmanga) |
| `-C`, `--clear-cache`        | キャッシュディレクトリを削除 (C:\Users\Administrator\AppData\Local\Temp\.cache\goreadmanga) |
| `-f`, `--fix`                | 問題を引き起こしているJSONエントリを削除（ネットワーク問題時の空のchapter_page/chapter_title） |

*注意: キャッシュディレクトリのパスは例です; アプリケーションはデフォルトでOSの一時ディレクトリを使用します。*


Disclaimer: for personnel and edumucational porpoises only.
