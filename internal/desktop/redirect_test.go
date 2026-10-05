package desktop

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFollowRedirectsRewritesRedirects(t *testing.T) {
	handler := FollowRedirects(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/invoices/7?x=</script>", http.StatusFound)
	}))

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/invoices", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if res.Header().Get("Location") != "" {
		t.Error("Location header should be removed")
	}
	body := res.Body.String()
	if !strings.Contains(body, `location.replace("/invoices/7?x=\u003c/script\u003e")`) {
		t.Errorf("unexpected bounce page: %s", body)
	}
	if strings.Contains(body, "Found") {
		t.Error("the original redirect body leaked through")
	}
}

func TestFollowRedirectsPassesThroughEverythingElse(t *testing.T) {
	handler := FollowRedirects(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte("form errors"))
	}))

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/invoices", nil))

	if res.Code != http.StatusUnprocessableEntity || res.Body.String() != "form errors" {
		t.Errorf("got %d %q", res.Code, res.Body.String())
	}
}

func TestDataDirIsCreated(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("AppData", home)

	dir, err := DataDir("Invoices")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dir, home) || !strings.HasSuffix(dir, "Invoices") {
		t.Errorf("DataDir = %s, expected it under %s", dir, home)
	}
}
