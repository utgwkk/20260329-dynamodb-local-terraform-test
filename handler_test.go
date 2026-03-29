package main

import (
	"embed"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

//go:embed testdata
var testdata embed.FS

func setupFixtureTestServer(t *testing.T, filename string) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := testdata.Open("testdata/" + filename)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
		}
		if _, err := io.Copy(w, f); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("DYNAMODB_LOCAL_HOST", strings.TrimPrefix(srv.URL, "http://"))
}

func TestProxyDynamoDBLocalHandler(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		setupFixtureTestServer(t, "DescribeTable_OK.json")

		rec := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(
			t.Context(), http.MethodPost, "http://localhost:8001", strings.NewReader(`{"TableName": "test"}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Add("Content-Type", "application/x-amz-json-1.0")
		req.Header.Add("X-Amz-Date", "20260329T110018Z")
		req.Header.Add("X-Amz-Target", "DynamoDB_20120810.DescribeTable")

		proxyDynamoDBLocalHandler(rec, req)

		if statusCode := rec.Result().StatusCode; statusCode != http.StatusOK {
			t.Errorf("unexpected status code: %d", statusCode)
		}
		var data struct {
			Table map[string]any `json:"Table"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&data); err != nil {
			t.Fatalf("failed to decode JSON: %s", err.Error())
		}
		if _, ok := data.Table["WarmThroughput"]; !ok {
			t.Error("response body does not have WarmThroughput field")
		}
	})
}
