package files

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/config"
)

func TestMinIOPutFailsClosedWhenBucketIsMissing(t *testing.T) {
	requestCount := 0
	mutationCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Method == http.MethodGet && r.URL.Path == "/missing-bucket/" && r.URL.Query().Has("location") {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/">us-east-1</LocationConstraint>`))
			return
		}
		if r.Method != http.MethodHead || r.URL.Path != "/missing-bucket/" {
			mutationCount++
			t.Fatalf("unexpected MinIO request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	storage, err := NewMinIOObjectStorage(config.MinIOConfig{
		Endpoint:  strings.TrimPrefix(server.URL, "http://"),
		AccessKey: "test-app-access",
		SecretKey: "test-app-secret",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = storage.Put(context.Background(), "missing-bucket", "answer.pdf", bytes.NewReader([]byte("answer")), 6, "application/pdf")
	if !errors.Is(err, ErrBucketMissing) {
		t.Fatalf("expected ErrBucketMissing, got %v", err)
	}
	if requestCount != 2 || mutationCount != 0 {
		t.Fatalf("missing bucket must only trigger location and existence checks; got %d requests and %d mutations", requestCount, mutationCount)
	}
}
