package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/nkenji09/product-memory/internal/viewer"
)

func newViewCmd() *cobra.Command {
	var port int
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

			// ローカル専用ビューア（§7）につき 127.0.0.1 のみで listen する（外部公開しない）。
			srv := &http.Server{Addr: "127.0.0.1:" + strconv.Itoa(resolvedPort), Handler: handler}
			fmt.Fprintf(cmd.OutOrStdout(), "pmem view: http://localhost:%d\n", resolvedPort)

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()

			errCh := make(chan error, 1)
			go func() { errCh <- srv.ListenAndServe() }()

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
	return cmd
}
