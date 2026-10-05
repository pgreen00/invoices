// Package web is the Gin application: routes, middleware, and rendering.
package web

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"invoices/internal/store"
)

const maxBodyBytes = 2 << 20

var mimeTypes = map[string]string{
	".css": "text/css; charset=utf-8",
	".js":  "text/javascript; charset=utf-8",
	".svg": "image/svg+xml",
	".png": "image/png",
	".ico": "image/x-icon",
}

// Files that browsers expect at the site root.
var rootFiles = []string{"favicon.ico", "icon.svg"}

type server struct {
	store  *store.Store
	pages  *renderer
	public fs.FS
}

// httpError carries a status code to the error page.
type httpError struct {
	status  int
	message string
}

func (e *httpError) Error() string { return e.message }

func notFound(format string, args ...any) error {
	return &httpError{status: http.StatusNotFound, message: fmt.Sprintf(format, args...)}
}

func init() {
	gin.SetMode(gin.ReleaseMode)
}

// New builds the Gin engine. The store must already be open.
func New(st *store.Store, views, public fs.FS) (*gin.Engine, error) {
	pages, err := newRenderer(views)
	if err != nil {
		return nil, err
	}
	s := &server{store: st, pages: pages, public: public}

	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.Use(recoverer(), formBody())

	r.GET("/assets/*filepath", s.asset)
	for _, name := range rootFiles {
		r.GET("/"+name, s.sendFile(name))
	}

	r.GET("/", s.handle(s.listInvoices))
	r.GET("/invoices/new", s.handle(s.newInvoice))
	r.POST("/invoices", s.handle(s.createInvoice))
	r.GET("/invoices/:id/edit", s.handle(s.editInvoice))
	r.POST("/invoices/:id", s.handle(s.updateInvoice))
	r.POST("/invoices/:id/delete", s.handle(s.deleteInvoice))
	r.GET("/invoices/:id", s.handle(s.printInvoice))

	r.GET("/clients", s.handle(s.listClients))
	r.GET("/clients/new", s.handle(s.newClient))
	r.POST("/clients", s.handle(s.createClient))
	r.GET("/clients/:id/edit", s.handle(s.editClient))
	r.POST("/clients/:id", s.handle(s.updateClient))
	r.POST("/clients/:id/delete", s.handle(s.deleteClient))

	r.GET("/settings", s.handle(s.showSettings))
	r.POST("/settings", s.handle(s.saveSettings))

	r.NoRoute(func(c *gin.Context) {
		errorPage(c, http.StatusNotFound, "Not Found")
	})
	r.NoMethod(func(c *gin.Context) {
		errorPage(c, http.StatusMethodNotAllowed, "Method Not Allowed")
	})

	return r, nil
}

// handle adapts an error-returning handler, rendering failures as the error
// page.
func (s *server) handle(fn func(*gin.Context) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		err := fn(c)
		if err == nil {
			return
		}

		var he *httpError
		if errors.As(err, &he) {
			errorPage(c, he.status, he.message)
			return
		}
		log.Printf("Unhandled error: %v", err)
		errorPage(c, http.StatusInternalServerError, err.Error())
	}
}

// recoverer turns a panic into the error page rather than a dropped
// response.
func recoverer() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("Unhandled panic: %v", recovered)
				errorPage(c, http.StatusInternalServerError, fmt.Sprint(recovered))
				c.Abort()
			}
		}()
		c.Next()
	}
}

// formBody parses urlencoded form bodies up front, capping their size. Line
// items are submitted as parallel arrays (item_hours, item_rate, ...), which
// url.Values represents directly.
func formBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)
			if err := c.Request.ParseForm(); err != nil {
				var tooLarge *http.MaxBytesError
				if errors.As(err, &tooLarge) {
					errorPage(c, http.StatusRequestEntityTooLarge, "Request body too large")
				} else {
					errorPage(c, http.StatusBadRequest, err.Error())
				}
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

func (s *server) writeFile(c *gin.Context, name string) bool {
	data, err := fs.ReadFile(s.public, name)
	if err != nil {
		return false
	}
	contentType, ok := mimeTypes[path.Ext(name)]
	if !ok {
		contentType = "application/octet-stream"
	}
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, contentType, data)
	return true
}

// asset serves /assets/* from the public directory. fs.ValidPath rejects
// any attempt to climb out of it.
func (s *server) asset(c *gin.Context) {
	name := strings.TrimLeft(c.Param("filepath"), "/")
	if name == "" || !fs.ValidPath(name) || !s.writeFile(c, name) {
		errorPage(c, http.StatusNotFound, "Not Found")
	}
}

func (s *server) sendFile(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.writeFile(c, name) {
			errorPage(c, http.StatusNotFound, "Not Found")
		}
	}
}

// parseID reads a numeric id. ok is false for anything that is not one, which
// callers treat the same as an id that does not exist.
func parseID(raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	return id, err == nil
}
