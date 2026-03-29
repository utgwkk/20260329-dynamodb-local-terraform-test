package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

func proxyDynamoDBLocalHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cloneReq := r.Clone(r.Context())
	cloneReq.RequestURI = ""
	cloneReq.URL.Scheme = "http"
	cloneReq.URL.Host = getEnvOrDefault("DYNAMODB_LOCAL_HOST", "localhost:8000")

	slog.DebugContext(
		ctx, "attempting to request",
		slog.String("host", cloneReq.Host),
		slog.String("method", cloneReq.Method),
		slog.String("path", cloneReq.URL.Path),
		slog.String("xAmzTarget", cloneReq.Header.Get("X-Amz-Target")),
	)

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
		if k == "Content-Length" {
			continue
		}
		for _, h := range headers {
			w.Header().Add(k, h)
		}
	}
	if strings.HasSuffix(cloneReq.Header.Get("X-Amz-Target"), ".DescribeTable") && proxyResp.StatusCode == http.StatusOK {
		slog.InfoContext(ctx, "attempting rewrite response JSON")
		data := map[string]map[string]any{}
		if err := json.NewDecoder(proxyResp.Body).Decode(&data); err != nil {
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

		w.WriteHeader(proxyResp.StatusCode)
		if err := json.NewEncoder(w).Encode(data); err != nil {
			slog.ErrorContext(ctx, "failed to write", slog.Any("error", err))
			return
		}
		slog.InfoContext(ctx, "response rewrite succeeded")
	} else {
		w.WriteHeader(proxyResp.StatusCode)
		io.Copy(w, proxyResp.Body)
	}
}
