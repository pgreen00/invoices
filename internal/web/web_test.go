package web

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"invoices/internal/store"
	"invoices/public"
	"invoices/views"
)

var (
	engine *gin.Engine
	db     *store.Store
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "invoices-test-")
	if err != nil {
		panic(err)
	}
	db, err = store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		panic(err)
	}
	engine, err = New(db, views.FS, public.FS)
	if err != nil {
		panic(err)
	}

	code := m.Run()
	db.Close()
	os.RemoveAll(dir)
	os.Exit(code)
}

func get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	res := httptest.NewRecorder()
	engine.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
	return res
}

func post(t *testing.T, path string, fields [][2]string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	for _, f := range fields {
		form.Add(f[0], f[1])
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	engine.ServeHTTP(res, req)
	return res
}

func mustMatch(t *testing.T, body, pattern string) {
	t.Helper()
	if !regexp.MustCompile(pattern).MatchString(body) {
		t.Errorf("expected body to match %s", pattern)
	}
}

func year() string {
	return strconv.Itoa(time.Now().Year())
}

/* ------------------------------------------------------------------ */
/* Pages                                                               */
/* ------------------------------------------------------------------ */

func TestPagesRender(t *testing.T) {
	for _, path := range []string{"/", "/invoices/new", "/clients", "/clients/new", "/settings"} {
		res := get(t, path)
		if res.Code != http.StatusOK {
			t.Errorf("GET %s → %d", path, res.Code)
		}
		if !strings.Contains(res.Header().Get("Content-Type"), "text/html") {
			t.Errorf("GET %s content type %q", path, res.Header().Get("Content-Type"))
		}
	}
}

func TestServesStaticAssets(t *testing.T) {
	for _, asset := range []string{"/assets/app.css", "/assets/print.css", "/assets/app.js"} {
		if res := get(t, asset); res.Code != http.StatusOK {
			t.Errorf("GET %s → %d", asset, res.Code)
		}
	}
}

func TestBlocksPathTraversalOnAssets(t *testing.T) {
	for _, path := range []string{"/assets/../internal/store/db.go", "/assets/%2e%2e/main.go", "/assets/"} {
		if res := get(t, path); res.Code == http.StatusOK {
			t.Errorf("GET %s → 200", path)
		}
	}
}

func TestServesIconsFromTheSiteRoot(t *testing.T) {
	expected := map[string]string{
		"/favicon.ico": "image/x-icon",
		"/icon.svg":    "image/svg+xml",
	}
	for path, contentType := range expected {
		res := get(t, path)
		if res.Code != http.StatusOK {
			t.Errorf("GET %s → %d", path, res.Code)
		}
		if !strings.Contains(res.Header().Get("Content-Type"), contentType) {
			t.Errorf("GET %s content type %q", path, res.Header().Get("Content-Type"))
		}
	}
}

func TestOnlyServesAllowListedFilesFromTheRoot(t *testing.T) {
	// public/app.css is reachable at /assets/app.css but must not leak to /.
	if res := get(t, "/app.css"); res.Code != http.StatusNotFound {
		t.Errorf("GET /app.css → %d", res.Code)
	}
}

func TestTitlesPagesWithoutRepeatingTheAppName(t *testing.T) {
	mustMatch(t, get(t, "/").Body.String(), `<title>Invoices</title>`)
	mustMatch(t, get(t, "/invoices/new").Body.String(), `<title>New invoice — Invoices</title>`)
}

func TestLinksTheIconsFromTheLayout(t *testing.T) {
	mustMatch(t, get(t, "/").Body.String(), `<link rel="icon" href="/icon.svg"`)
}

func TestMarksTheActiveNavLink(t *testing.T) {
	mustMatch(t, get(t, "/clients").Body.String(), `<a href="/clients" class="active">Clients</a>`)
	mustMatch(t, get(t, "/").Body.String(), `<a href="/clients" class="">Clients</a>`)
}

func Test404sUnknownRoutes(t *testing.T) {
	res := get(t, "/nope")
	if res.Code != http.StatusNotFound {
		t.Errorf("GET /nope → %d", res.Code)
	}
	mustMatch(t, res.Body.String(), `Back to invoices`)
}

func TestPrefillsTheNewInvoiceFormWithFiveWeekdayRows(t *testing.T) {
	body := get(t, "/invoices/new").Body.String()
	if n := strings.Count(body, `name="item_date"`); n != 5+1 { // 5 rows + <template>
		t.Errorf("found %d item_date inputs", n)
	}
	mustMatch(t, body, `Northwind Retail Group`)
	mustMatch(t, body, `<option value="1" selected>Northwind`)
	mustMatch(t, body, `"default_rate":"165.00","default_terms_days":15`)
}

/* ------------------------------------------------------------------ */
/* Invoice lifecycle                                                   */
/* ------------------------------------------------------------------ */

func TestInvoiceLifecycle(t *testing.T) {
	var invoiceURL string

	t.Run("creates an invoice and auto-numbers it", func(t *testing.T) {
		res := post(t, "/invoices", [][2]string{
			{"client_id", "1"},
			{"number", ""},
			{"project_name", "Order Management Platform"},
			{"invoice_date", "2024-03-24"},
			{"period_start", "2024-03-18"},
			{"period_end", "2024-03-22"},
			{"terms_days", "15"},
			{"expenses", "0"},
			{"discount", "0"},
			{"item_date", "2024-03-18"},
			{"item_description", "Checkout service refactor"},
			{"item_detail", "Extracted payment orchestration."},
			{"item_hours", "8"},
			{"item_rate", "165.00"},
			{"item_date", "2024-03-23"},
			{"item_description", "Emergency production hotfix"},
			{"item_detail", ""},
			{"item_hours", "2"},
			{"item_rate", "247.50"},
			// An untouched trailing row that should be discarded.
			{"item_date", "2024-03-25"},
			{"item_description", ""},
			{"item_detail", ""},
			{"item_hours", ""},
			{"item_rate", "165.00"},
			{"payment_instructions", "ACH / Wire — Lone Star Bank"},
			{"notes", "Net 15."},
			{"footer_message", "Thank you for your business."},
		})

		if res.Code != http.StatusFound {
			t.Fatalf("status %d: %s", res.Code, res.Body.String())
		}
		invoiceURL = res.Header().Get("Location")
		mustMatch(t, invoiceURL, `^/invoices/\d+$`)

		body := get(t, invoiceURL).Body.String()
		mustMatch(t, body, `No\. `+year()+`-001`)
		mustMatch(t, body, `\$1,815\.00`) // 8×165 + 2×247.50
		mustMatch(t, body, `10\.00`)      // billable hours
		mustMatch(t, body, `Apr 8, 2024`) // due date = invoice date + 15
		mustMatch(t, body, `Mar 18 – 22, 2024`)
		if strings.Contains(body, "Mar 25") {
			t.Error("blank row was not dropped")
		}
	})

	t.Run("refers to the invoice number in the payment instructions", func(t *testing.T) {
		mustMatch(t, get(t, invoiceURL).Body.String(), `<p>Reference: Invoice `+year()+`-001</p>`)
	})

	t.Run("prefills saved hours when editing", func(t *testing.T) {
		body := get(t, invoiceURL+"/edit").Body.String()
		mustMatch(t, body, `name="item_hours" class="right" value="8"`)
		mustMatch(t, body, `name="item_hours" class="right" value="2"`)
	})

	t.Run("prints the phone above the email, each on its own line", func(t *testing.T) {
		mustMatch(t, get(t, invoiceURL).Body.String(),
			`<div>\(512\) 555-0142</div>\s*<div>billing@ravenline\.dev</div>`)
	})

	t.Run("shows the invoice in the list with computed totals", func(t *testing.T) {
		body := get(t, "/").Body.String()
		mustMatch(t, body, `\$1,815\.00`)
		mustMatch(t, body, `Northwind Retail Group`)
		mustMatch(t, body, `1 invoice ·`)
	})

	t.Run("rejects an invoice with no client or line items", func(t *testing.T) {
		res := post(t, "/invoices", [][2]string{
			{"client_id", ""},
			{"invoice_date", "2024-03-24"},
			{"terms_days", "15"},
		})
		if res.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status %d", res.Code)
		}
		mustMatch(t, res.Body.String(), `Select a client`)
		mustMatch(t, res.Body.String(), `at least one line item`)
	})

	t.Run("rejects a duplicate invoice number", func(t *testing.T) {
		res := post(t, "/invoices", [][2]string{
			{"client_id", "1"},
			{"number", year() + "-001"},
			{"invoice_date", "2024-03-31"},
			{"terms_days", "15"},
			{"item_description", "Work"},
			{"item_hours", "1"},
			{"item_rate", "100"},
		})
		if res.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status %d", res.Code)
		}
		mustMatch(t, res.Body.String(), `already in use`)
	})

	t.Run("edits an invoice and recomputes the total", func(t *testing.T) {
		id := invoiceURL[strings.LastIndex(invoiceURL, "/")+1:]
		res := post(t, "/invoices/"+id, [][2]string{
			{"client_id", "1"},
			{"number", ""},
			{"invoice_date", "2024-03-24"},
			{"period_start", "2024-03-18"},
			{"period_end", "2024-03-22"},
			{"terms_days", "30"},
			{"expenses", "125.50"},
			{"discount", "25"},
			{"item_date", "2024-03-18"},
			{"item_description", "Checkout service refactor"},
			{"item_hours", "9"},
			{"item_rate", "165.00"},
		})
		if res.Code != http.StatusFound {
			t.Fatalf("status %d", res.Code)
		}

		body := get(t, invoiceURL).Body.String()
		mustMatch(t, body, `\$1,585\.50`)  // 9×165 + 125.50 − 25
		mustMatch(t, body, `Apr 23, 2024`) // terms now Net 30
		mustMatch(t, body, `Net 30`)
	})

	t.Run("preserves generated_at while bumping updated_at", func(t *testing.T) {
		var generated, updated string
		if err := db.DB().QueryRow(`SELECT generated_at, updated_at FROM invoices LIMIT 1`).Scan(&generated, &updated); err != nil {
			t.Fatal(err)
		}
		if updated < generated {
			t.Errorf("updated_at %s < generated_at %s", updated, generated)
		}
	})

	t.Run("rolls an invoice forward a week via Repeat", func(t *testing.T) {
		id := invoiceURL[strings.LastIndex(invoiceURL, "/")+1:]
		body := get(t, "/invoices/new?copy_from="+id).Body.String()
		mustMatch(t, body, `value="2024-03-25"`) // period start + 7 days
		mustMatch(t, body, `value="2024-03-29"`) // period end + 7 days
		mustMatch(t, body, `Checkout service refactor`)
		mustMatch(t, body, `name="item_hours" class="right" value=""`) // hours cleared
	})

	t.Run("404s a Repeat of a missing invoice", func(t *testing.T) {
		res := get(t, "/invoices/new?copy_from=999")
		if res.Code != http.StatusNotFound {
			t.Errorf("status %d", res.Code)
		}
		mustMatch(t, res.Body.String(), `No invoice with id 999\.`)
	})

	t.Run("numbers the second invoice sequentially", func(t *testing.T) {
		res := post(t, "/invoices", [][2]string{
			{"client_id", "1"},
			{"number", ""},
			{"invoice_date", "2024-03-31"},
			{"terms_days", "15"},
			{"item_description", "Follow-up work"},
			{"item_hours", "4"},
			{"item_rate", "165"},
		})
		if res.Code != http.StatusFound {
			t.Fatalf("status %d", res.Code)
		}
		mustMatch(t, get(t, res.Header().Get("Location")).Body.String(), `No\. `+year()+`-002`)
	})

	t.Run("deletes an invoice and cascades its line items", func(t *testing.T) {
		id := invoiceURL[strings.LastIndex(invoiceURL, "/")+1:]
		if res := post(t, "/invoices/"+id+"/delete", nil); res.Code != http.StatusFound {
			t.Fatalf("delete status %d", res.Code)
		}
		if res := get(t, invoiceURL); res.Code != http.StatusNotFound {
			t.Errorf("deleted invoice → %d", res.Code)
		}

		var orphans int
		if err := db.DB().QueryRow(`SELECT COUNT(*) FROM line_items WHERE invoice_id = ?`, id).Scan(&orphans); err != nil {
			t.Fatal(err)
		}
		if orphans != 0 {
			t.Errorf("%d orphaned line items", orphans)
		}
	})
}

/* ------------------------------------------------------------------ */
/* Stylesheets                                                         */
/* ------------------------------------------------------------------ */

func readCSS(t *testing.T, name string) string {
	t.Helper()
	data, err := fs.ReadFile(public.FS, name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// This guards a bug that is invisible on screen: the @media print block and
// the screen rules it overrides have equal specificity, so if the print block
// is moved above them it silently loses the cascade and the page background,
// drop shadow and doubled padding all end up on paper.
func TestPrintStylesheetDeclaresOverridesAfterScreenRules(t *testing.T) {
	css := readCSS(t, "print.css")
	printAt := strings.Index(css, "@media print")
	if printAt < 0 {
		t.Fatal("expected an @media print block")
	}
	if printAt < strings.Index(css, "body {") || printAt < strings.Index(css, ".sheet {") {
		t.Error("print block must come after body and .sheet")
	}
}

func TestPrintStylesheetPinsInvoicesToLight(t *testing.T) {
	// The app follows the system scheme, but an invoice is a printed document
	// and must never render dark.
	css := readCSS(t, "print.css")
	mustMatch(t, css, `color-scheme:\s*light;`)
	if regexp.MustCompile(`color-scheme:\s*light dark|light-dark\(`).MatchString(css) {
		t.Error("print.css must not follow the system colour scheme")
	}
}

func TestPrintStylesheetStripsScreenChrome(t *testing.T) {
	css := readCSS(t, "print.css")
	printBlock := css[strings.Index(css, "@media print"):]
	for _, pattern := range []string{`box-shadow:\s*none`, `margin:\s*0;`, `padding:\s*0;`, `display:\s*none\s*!important`} {
		mustMatch(t, printBlock, pattern)
	}
}

func TestAppStylesheetFollowsSystemColourScheme(t *testing.T) {
	mustMatch(t, readCSS(t, "app.css"), `color-scheme:\s*light dark;`)
}

func TestAppStylesheetHasNoBareLightModeColours(t *testing.T) {
	css := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(readCSS(t, "app.css"), "")
	colour := regexp.MustCompile(`#[0-9a-fA-F]{3,8}|rgba?\(`)
	// The nav bar is dark in both schemes, so its white text is correct in
	// both and must not be wrapped.
	navText := regexp.MustCompile(`#fff|#aeb6bf`)

	for i, line := range strings.Split(css, "\n") {
		if colour.MatchString(line) && !strings.Contains(line, "light-dark(") && !navText.MatchString(line) {
			t.Errorf("unwrapped colour on line %d: %s", i+1, strings.TrimSpace(line))
		}
	}
}

/* ------------------------------------------------------------------ */
/* Clients and settings                                                */
/* ------------------------------------------------------------------ */

func TestClientsAndSettings(t *testing.T) {
	t.Run("creates a client and offers it on the invoice form", func(t *testing.T) {
		res := post(t, "/clients", [][2]string{
			{"name", "Contoso Logistics"},
			{"email", "ap@contoso.test"},
			{"default_rate", "185.00"},
			{"default_terms_days", "30"},
		})
		if res.Code != http.StatusFound {
			t.Fatalf("status %d", res.Code)
		}
		mustMatch(t, get(t, "/invoices/new").Body.String(), `Contoso Logistics`)
	})

	t.Run("rejects a client with no name", func(t *testing.T) {
		res := post(t, "/clients", [][2]string{{"name", "  "}, {"default_rate", "12.5"}})
		if res.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status %d", res.Code)
		}
		mustMatch(t, res.Body.String(), `Client name is required`)
		mustMatch(t, res.Body.String(), `name="default_rate" value="12.5"`)
	})

	t.Run("archives a client and hides it from the invoice form", func(t *testing.T) {
		var id int64
		if err := db.DB().QueryRow(`SELECT id FROM clients WHERE name = 'Contoso Logistics'`).Scan(&id); err != nil {
			t.Fatal(err)
		}
		res := post(t, fmt.Sprintf("/clients/%d", id), [][2]string{{"name", "Contoso Logistics"}, {"archived", "1"}})
		if res.Code != http.StatusFound {
			t.Fatalf("status %d", res.Code)
		}
		if strings.Contains(get(t, "/invoices/new").Body.String(), "Contoso Logistics") {
			t.Error("archived client still offered")
		}
		mustMatch(t, get(t, "/clients").Body.String(), `<span class="tag">archived</span>`)
	})

	t.Run("saves settings without rewriting past invoices", func(t *testing.T) {
		var before string
		if err := db.DB().QueryRow(`SELECT business_name FROM invoices LIMIT 1`).Scan(&before); err != nil {
			t.Fatal(err)
		}

		res := post(t, "/settings", [][2]string{
			{"business_name", "Renamed Software LLC"},
			{"default_rate", "200.00"},
			{"default_terms_days", "20"},
		})
		if res.Code != http.StatusFound || res.Header().Get("Location") != "/settings?saved=1" {
			t.Fatalf("status %d → %s", res.Code, res.Header().Get("Location"))
		}

		settings, err := db.Settings()
		if err != nil {
			t.Fatal(err)
		}
		if settings.BusinessName != "Renamed Software LLC" || settings.DefaultRateCents != 20000 {
			t.Errorf("settings = %+v", settings)
		}

		var after string
		if err := db.DB().QueryRow(`SELECT business_name FROM invoices LIMIT 1`).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if after != before {
			t.Errorf("past invoice business name changed from %q to %q", before, after)
		}
	})

	t.Run("rejects settings with no business name", func(t *testing.T) {
		res := post(t, "/settings", [][2]string{{"business_name", ""}})
		if res.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status %d", res.Code)
		}
		mustMatch(t, res.Body.String(), `Business name is required`)
	})
}

func TestRejectsOversizedBodies(t *testing.T) {
	res := post(t, "/clients", [][2]string{{"name", strings.Repeat("x", maxBodyBytes+1)}})
	if res.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status %d", res.Code)
	}
}
