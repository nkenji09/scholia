module github.com/nkenji09/scholia

go 1.26.5

require github.com/spf13/cobra v1.10.2

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)

retract [v0.1.0, v0.2.3] // 旧リリースが顧客識別子を embed（v0.2.3 は plugin.json 不整合）。v0.2.4 以降を使用
