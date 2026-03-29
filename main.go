package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/ne-sachirou/go-graceful"
	"github.com/ne-sachirou/go-graceful/gracefulhttp"
)

func main() {
	ctx := context.Background()

	slog.SetDefault(
		slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		})),
	)

	http.HandleFunc("/", proxyDynamoDBLocalHandler)

	if err := gracefulhttp.ListenAndServe(
		ctx,
		":8001",
		nil,
		graceful.GracefulShutdownTimeout(10*time.Second),
	); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.ErrorContext(ctx, "failed to listen", slog.Any("error", err))
	}
}
