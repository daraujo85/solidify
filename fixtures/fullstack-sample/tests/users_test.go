package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"fixture/api"
)

func TestHandleUsers(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	api.HandleUsers(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var u map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&u); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if u["name"] != "fixture-user" {
		t.Errorf("name: %s", u["name"])
	}
}
