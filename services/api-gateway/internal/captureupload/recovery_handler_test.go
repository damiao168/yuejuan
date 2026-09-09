package captureupload

import (
	"edugrade-enterprise/services/api-gateway/internal/auth"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestRecoveryHandlerReturnsCommandAndReadOnlyCheckpoint(t *testing.T) {
	ctx, service, _, _, _, batchID := newTestService(t)
	payload := append([]byte("\x89PNG\r\n\x1a\n"), []byte("recovery")...)
	input := testInitInput(payload, batchID)
	initialized, err := service.Init(ctx, testTenant, testActor, input)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(service, auth.NewMemoryStore())
	read := func(want string) {
		t.Helper()
		before, err := service.Get(ctx, testTenant, initialized.RemoteUploadID)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest("GET", "/recover", nil)
		request.SetPathValue("id", initialized.RemoteUploadID)
		request = request.WithContext(auth.WithAccessScope(auth.WithUser(ctx, auth.User{ID: testActor, TenantID: testTenant}), auth.AccessScope{TenantID: testTenant, TenantWide: true}))
		response := httptest.NewRecorder()
		handler.Get(response, request)
		var result struct {
			CommandID string  `json:"command_id"`
			Status    string  `json:"status"`
			Upload    Session `json:"upload"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 || result.CommandID != input.IdempotencyKey || result.Status != want || result.Upload.ConfirmedOffset != before.ConfirmedOffset {
			t.Fatalf("recovery %d %s err=%v", response.Code, response.Body.String(), err)
		}
		after, err := service.Get(ctx, testTenant, initialized.RemoteUploadID)
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatal("GET changed durable upload state")
		}
	}
	read("processing")
	if _, err := service.AppendChunk(ctx, testTenant, initialized.RemoteUploadID, ChunkInput{Offset: 0, SHA256: sha256Hex(payload), Data: payload}); err != nil {
		t.Fatal(err)
	}
	read("processing")
	if _, err := service.Complete(ctx, testTenant, testActor, initialized.RemoteUploadID, CompleteInput{SHA256: input.SHA256}); err != nil {
		t.Fatal(err)
	}
	read("succeeded")
	request := httptest.NewRequest("GET", "/recover", nil)
	request.SetPathValue("id", initialized.RemoteUploadID)
	request = request.WithContext(auth.WithAccessScope(auth.WithUser(ctx, auth.User{ID: testActor, TenantID: testTenant}), auth.AccessScope{TenantID: testTenant}))
	response := httptest.NewRecorder()
	handler.Get(response, request)
	if response.Code != 403 {
		t.Fatalf("unscoped recovery returned %d", response.Code)
	}
}
