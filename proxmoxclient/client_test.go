package proxmoxclient

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/version" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte(`{"data":{"version":"7.1-10","release":"2021-11-23"}}`))
	}))
	defer ts.Close()

	client := New(ts.URL)
	resp, err := client.Version()
	if err != nil {
		require.NoError(t, err, "Version failed")
	}
	assert.Equal(t, `{"data":{"version":"7.1-10","release":"2021-11-23"}}`, resp)
}
