package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"invoices/internal/dates"
	"invoices/internal/jscompat"
	"invoices/internal/money"
)

const appName = "Invoices"

var funcs = template.FuncMap{
	"money":     money.FormatCents,
	"hours":     money.FormatHours,
	"dateLong":  dates.FormatLong,
	"dateDay":   dates.FormatDayDate,
	"dateRange": dates.FormatRange,
	"timestamp": dates.FormatTimestamp,

	// lines splits multi-line text so templates can emit one node per line.
	"lines": func(text string) []string {
		var out []string
		for _, line := range strings.Split(text, "\n") {
			if line = jscompat.Trim(line); line != "" {
				out = append(out, line)
			}
		}
		return out
	},

	// jsonScript embeds a value in a <script type="application/json"> block.
	// encoding/json already escapes <, > and & so it cannot close the tag.
	"jsonScript": func(value any) (template.JS, error) {
		data, err := json.Marshal(value)
		return template.JS(data), err
	},
}

// Pages rendered inside layout.html. Each defines a "content" template.
var layoutPages = []string{
	"invoices/list",
	"invoices/form",
	"clients/list",
	"clients/form",
	"settings/form",
}

// Pages that are complete documents on their own.
var standalonePages = []string{
	"invoices/print",
}

type renderer struct {
	pages      map[string]*template.Template
	standalone map[string]bool
}

func newRenderer(views fs.FS) (*renderer, error) {
	r := &renderer{pages: map[string]*template.Template{}, standalone: map[string]bool{}}

	for _, name := range layoutPages {
		t, err := template.New("layout.html").Funcs(funcs).Option("missingkey=zero").
			ParseFS(views, "layout.html", name+".html")
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", name, err)
		}
		r.pages[name] = t
	}

	for _, name := range standalonePages {
		base := name[strings.LastIndex(name, "/")+1:] + ".html"
		t, err := template.New(base).Funcs(funcs).Option("missingkey=zero").ParseFS(views, name+".html")
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", name, err)
		}
		r.pages[name] = t
		r.standalone[name] = true
	}

	return r, nil
}

// render executes page name with data and writes it with status. Pages are
// rendered to a buffer first so a template error becomes a clean 500 rather
// than half a page.
func (r *renderer) render(c *gin.Context, status int, name string, data gin.H) error {
	t, ok := r.pages[name]
	if !ok {
		return fmt.Errorf("no template named %q", name)
	}

	title, _ := data["Title"].(string)
	if title == "" {
		title = appName
	}
	data["Title"] = title
	data["Path"] = c.Request.URL.Path
	// Avoids a redundant "Invoices — Invoices" on the index page.
	if title == appName {
		data["DocumentTitle"] = appName
	} else {
		data["DocumentTitle"] = title + " — " + appName
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return err
	}
	c.Data(status, "text/html; charset=utf-8", buf.Bytes())
	return nil
}

// errorPage is shown for anything that goes wrong, instead of a bare stack
// trace.
func errorPage(c *gin.Context, status int, message string) {
	writeErrorPage(c.Writer, status, message)
}

// Unavailable serves the error page for every request. It stands in for the
// app when it cannot start, typically because the database will not open, so
// the window explains the problem instead of staying blank.
func Unavailable(err error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeErrorPage(w, http.StatusInternalServerError, err.Error())
	})
}

func writeErrorPage(w http.ResponseWriter, status int, message string) {
	heading := "Something went wrong"
	if status == http.StatusNotFound {
		heading = "Not found"
	}
	body := fmt.Sprintf(`<!doctype html><meta charset="utf-8">
<title>%d — Invoices</title>
<style>
  body{font:14px/1.6 -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;
       max-width:44rem;margin:4rem auto;padding:0 1.5rem;color:#1c1f23}
  h1{font-size:1.25rem;margin:0 0 .75rem}
  pre{background:#f1f2f4;padding:1rem;border-radius:6px;overflow:auto;white-space:pre-wrap}
  a{color:#2563eb}
</style>
<h1>%s</h1>
<pre>%s</pre>
<p><a href="/">Back to invoices</a></p>`, status, heading, escapeHTML(message))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func escapeHTML(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(value)
}
