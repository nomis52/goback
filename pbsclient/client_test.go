package pbsclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPing(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/ping" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte(`{"data":"pong"}`))
	}))
	defer ts.Close()

	client := New(ts.URL)
	resp, err := client.Ping()
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
	if resp != `{"data":"pong"}` {
		t.Errorf("unexpected response: %s", resp)
	}
}
