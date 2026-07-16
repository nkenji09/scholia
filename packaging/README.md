# packaging

- **install.sh**: 利用可能。`curl -fsSL https://raw.githubusercontent.com/nkenji09/scholia/main/packaging/install.sh | sh` で、GitHub Releases から `scholia_<os>_<arch>.tar.gz` を取得し darwin/linux にインストールする。
- **goreleaser**: 稼働中。`v*` タグの push で release CI がビルドを走らせ、GitHub Releases にバイナリを生成する（`.goreleaser.yaml`）。
- **homebrew / scoop**: draft。`homebrew/scholia.rb` と `scoop/scholia.json` はまだ version/sha256 が placeholder で使えない。goreleaser の `brews:`/`scoops:` publisher を tap/bucket リポジトリに配線するまで有効化しない（別トラック）。
