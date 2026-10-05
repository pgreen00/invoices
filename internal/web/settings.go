package web

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"invoices/internal/forms"
	"invoices/internal/money"
	"invoices/internal/store"
)

// settingsForm is Settings as the form displays it.
type settingsForm struct {
	store.Settings
	DefaultRate      string
	DefaultTermsDays string // shadows the int64 in Settings
}

func (s *server) showSettings(c *gin.Context) error {
	settings, err := s.store.Settings()
	if err != nil {
		return err
	}
	return s.pages.render(c, http.StatusOK, "settings/form", gin.H{
		"Title": "Settings",
		"Settings": settingsForm{
			Settings:         settings,
			DefaultRate:      money.CentsToInput(settings.DefaultRateCents),
			DefaultTermsDays: itoa(settings.DefaultTermsDays),
		},
		"Saved": c.Query("saved") == "1",
	})
}

func (s *server) saveSettings(c *gin.Context) error {
	form := c.Request.PostForm
	values, errs := forms.ParseSettings(form)

	if len(errs) > 0 {
		return s.pages.render(c, http.StatusUnprocessableEntity, "settings/form", gin.H{
			"Title": "Settings",
			"Settings": settingsForm{
				Settings:         values,
				DefaultRate:      form.Get("default_rate"), // redisplayed exactly as typed
				DefaultTermsDays: itoa(values.DefaultTermsDays),
			},
			"Errors": errs,
			"Saved":  false,
		})
	}

	if err := s.store.SaveSettings(values); err != nil {
		return err
	}
	c.Redirect(http.StatusFound, "/settings?saved=1")
	return nil
}
