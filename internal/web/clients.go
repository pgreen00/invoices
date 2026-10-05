package web

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"invoices/internal/forms"
	"invoices/internal/money"
	"invoices/internal/store"
)

// clientForm is a client as the form displays it.
type clientForm struct {
	ID               int64
	Name             string
	Attn             string
	AddressLine1     string
	AddressLine2     string
	CityStateZip     string
	Email            string
	DefaultProject   string
	DefaultContract  string
	DefaultPO        string
	DefaultRate      string
	DefaultTermsDays string
	Archived         bool
}

func optionalInt(p *int64) string {
	if p == nil {
		return ""
	}
	return itoa(*p)
}

func toClientForm(id int64, c store.ClientData, rate string) clientForm {
	return clientForm{
		ID:               id,
		Name:             c.Name,
		Attn:             c.Attn,
		AddressLine1:     c.AddressLine1,
		AddressLine2:     c.AddressLine2,
		CityStateZip:     c.CityStateZip,
		Email:            c.Email,
		DefaultProject:   c.DefaultProject,
		DefaultContract:  c.DefaultContract,
		DefaultPO:        c.DefaultPO,
		DefaultRate:      rate,
		DefaultTermsDays: optionalInt(c.DefaultTermsDays),
		Archived:         c.Archived,
	}
}

func (s *server) renderClientForm(c *gin.Context, status int, title, action string, client clientForm, errs []string) error {
	return s.pages.render(c, status, "clients/form", gin.H{
		"Title":  title,
		"Action": action,
		"Client": client,
		"Errors": errs,
	})
}

func (s *server) listClients(c *gin.Context) error {
	clients, err := s.store.ListClients(true)
	if err != nil {
		return err
	}
	return s.pages.render(c, http.StatusOK, "clients/list", gin.H{
		"Title":   "Clients",
		"Clients": clients,
	})
}

func (s *server) newClient(c *gin.Context) error {
	return s.renderClientForm(c, http.StatusOK, "New client", "/clients", clientForm{}, nil)
}

func (s *server) createClient(c *gin.Context) error {
	form := c.Request.PostForm
	values, errs := forms.ParseClient(form)

	if len(errs) > 0 {
		// The rate is redisplayed exactly as typed.
		return s.renderClientForm(c, http.StatusUnprocessableEntity, "New client", "/clients",
			toClientForm(0, values, form.Get("default_rate")), errs)
	}

	if _, err := s.store.CreateClient(values); err != nil {
		return err
	}
	c.Redirect(http.StatusFound, "/clients")
	return nil
}

// existingClient finds the client named in the URL, or returns a 404.
func (s *server) existingClient(c *gin.Context) (*store.Client, error) {
	raw := c.Param("id")
	client, err := s.lookupClient(raw)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, notFound("No client with id %s.", raw)
	}
	return client, nil
}

func (s *server) editClient(c *gin.Context) error {
	client, err := s.existingClient(c)
	if err != nil {
		return err
	}

	rate := ""
	if client.DefaultRateCents != nil {
		rate = money.CentsToInput(*client.DefaultRateCents)
	}
	return s.renderClientForm(c, http.StatusOK, "Edit "+client.Name, "/clients/"+itoa(client.ID),
		toClientForm(client.ID, client.ClientData, rate), nil)
}

func (s *server) updateClient(c *gin.Context) error {
	client, err := s.existingClient(c)
	if err != nil {
		return err
	}

	form := c.Request.PostForm
	values, errs := forms.ParseClient(form)

	if len(errs) > 0 {
		return s.renderClientForm(c, http.StatusUnprocessableEntity, "Edit "+client.Name, "/clients/"+itoa(client.ID),
			toClientForm(client.ID, values, form.Get("default_rate")), errs)
	}

	if err := s.store.UpdateClient(client.ID, values); err != nil {
		return err
	}
	c.Redirect(http.StatusFound, "/clients")
	return nil
}

// deleteClient removes a client. Invoices keep their snapshotted billing
// details; the foreign key is set to NULL.
func (s *server) deleteClient(c *gin.Context) error {
	if id, ok := parseID(c.Param("id")); ok {
		if err := s.store.DeleteClient(id); err != nil {
			return err
		}
	}
	c.Redirect(http.StatusFound, "/clients")
	return nil
}
