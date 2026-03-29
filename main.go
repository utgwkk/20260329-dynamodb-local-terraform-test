package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ne-sachirou/go-graceful"
	"github.com/ne-sachirou/go-graceful/gracefulhttp"
	"github.com/thinkgos/httpcurl"
)

func main() {
	ctx := context.Background()

	slog.SetDefault(
		slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		})),
	)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		cloneReq := r.Clone(r.Context())
		cloneReq.Body = io.NopCloser(bytes.NewBuffer(rawBody))
		cloneReq.RequestURI = ""
		cloneReq.URL.Scheme = "http"
		cloneReq.URL.Host = "localhost:8000"

		slog.DebugContext(
			ctx, "attempting to request",
			slog.String("host", cloneReq.Host),
			slog.String("method", cloneReq.Method),
			slog.String("path", cloneReq.URL.Path),
			slog.String("xAmzTarget", cloneReq.Header.Get("X-Amz-Target")),
		)
		curlStr, err := httpcurl.IntoCurl(cloneReq)
		if err != nil {
			slog.ErrorContext(ctx, "failed to dump request as curl", slog.Any("error", err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Println(curlStr)

		proxyResp, err := http.DefaultClient.Do(cloneReq)
		if err != nil {
			slog.ErrorContext(ctx, "failed to request", slog.Any("error", err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer proxyResp.Body.Close()

		slog.DebugContext(
			ctx, "got an response",
			slog.String("status", proxyResp.Status),
		)

		for k, headers := range proxyResp.Header {
			for _, h := range headers {
				w.Header().Add(k, h)
			}
		}
		if strings.HasSuffix(cloneReq.Header.Get("X-Amz-Target"), ".DescribeTable") && proxyResp.StatusCode == http.StatusOK {
			slog.InfoContext(ctx, "attempting rewrite response JSON")
			data := map[string]map[string]any{}
			rawData, err := io.ReadAll(proxyResp.Body)
			if err != nil {
				slog.ErrorContext(ctx, "failed to read proxy response body", slog.Any("error", err))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if err := json.Unmarshal(rawData, &data); err != nil {
				slog.ErrorContext(ctx, "failed to decode JSON", slog.Any("error", err))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			slog.DebugContext(ctx, "raw response", slog.Any("data", data))

			// append dummy WarmThroughput
			data["Table"]["WarmThroughput"] = map[string]any{
				"Status":              "ACTIVE",
				"ReadUnitsPerSecond":  5,
				"WriteUnitsPerSecond": 5,
			}

			encoded, err := json.Marshal(data)
			if err != nil {
				slog.ErrorContext(ctx, "failed to encode JSON", slog.Any("error", err))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(proxyResp.StatusCode)
			_, err = w.Write(encoded)
			if err != nil {
				slog.ErrorContext(ctx, "failed to write", slog.Any("error", err))
			}
			slog.InfoContext(ctx, "response rewrite succeeded")
		} else {
			w.WriteHeader(proxyResp.StatusCode)
			io.Copy(w, proxyResp.Body)
		}
	})

	if err := gracefulhttp.ListenAndServe(
		ctx,
		":8001",
		nil,
		graceful.GracefulShutdownTimeout(10*time.Second),
	); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.ErrorContext(ctx, "failed to listen", slog.Any("error", err))
	}
}
