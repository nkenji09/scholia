package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/viewer"
)

// isLocalHost reports whether host is a loopback-only bind address — the
// default, and the boundary past which --host's LAN-exposure warning fires.
func isLocalHost(host string) bool {
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

// classifyListenErr turns a net.Listen failure into a user-facing error that
// states the cause and the next step (§T-view-start-port-in-use). Port
// contention gets a specific message; other bind failures (e.g. permission
// denied on a privileged port) fall back to a generic one.
func classifyListenErr(err error, port int) error {
	if errors.Is(err, syscall.EADDRINUSE) {
		return fmt.Errorf("port %d は使用中です。別の port を --port で指定するか、使用中のプロセスを止めてください", port)
	}
	return fmt.Errorf("port %d への bind に失敗しました: %w", port, err)
}

func newViewCmd() *cobra.Command {
	var port int
	var host string
	cmd := &cobra.Command{
		Use:   "view",
		Short: "ローカルビューアを起動する（埋め込み SPA・§7）",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}

			// port の優先順: --port > config.viewer.port > 4577（既定・DESIGN §6/§11）。
			resolvedPort := cfg.Viewer.Port
			if resolvedPort == 0 {
				resolvedPort = 4577
			}
			if cmd.Flags().Changed("port") {
				resolvedPort = port
			}

			handler, err := viewer.NewHandler(s)
			if err != nil {
				return err
			}

			// 既定はローカル専用（§7・127.0.0.1 のみで listen、外部公開しない）。
			// --host で任意アドレスへの opt-in 公開ができる（例: --host 0.0.0.0 で
			// LAN からスマホ等で見る）。config の唯一 CRUD 経路（PUT /api/config）も
			// 同じ listener に乗るため、ローカル以外へ bind するときは注意を出す。
			if !isLocalHost(host) {
				fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  %s でLANに公開します。設定編集 API（PUT /api/config）も同一ネットワークから到達可能になります。\n", host)
			}
			addr := host + ":" + strconv.Itoa(resolvedPort)
			// 先に bind してから URL を出す（§T-view-start-port-in-use・
			// 2026-07-15 decision）。bind 前に出力すると「URL が出た＝起動成功」
			// に見えた直後に port 使用中で落ちる紛らわしさが生じるため。
			listener, err := net.Listen("tcp", addr)
			if err != nil {
				return classifyListenErr(err, resolvedPort)
			}

			srv := &http.Server{Addr: addr, Handler: handler}
			// 表示用ホスト名: 既定(127.0.0.1)は従来通り "localhost" と表示（挙動不変）、
			// --host 指定時は実際の bind 先をそのまま出す。
			displayHost := host
			if host == "127.0.0.1" {
				displayHost = "localhost"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "scholia view: http://%s:%d\n", displayHost, resolvedPort)

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()

			errCh := make(chan error, 1)
			go func() { errCh <- srv.Serve(listener) }()

			select {
			case err := <-errCh:
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					return err
				}
				return nil
			case <-ctx.Done():
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				return srv.Shutdown(shutdownCtx)
			}
		},
	}
	cmd.Flags().IntVar(&port, "port", 0, "ポート番号（既定: config.viewer.port または 4577）")
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "listen する host（既定はローカル専用。0.0.0.0 等を指定すると LAN に公開— 設定編集 API も到達可能になるため注意）")
	return cmd
}
