package web

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"invoices/internal/dates"
	"invoices/internal/forms"
	"invoices/internal/money"
	"invoices/internal/store"
)

/* ------------------------------------------------------------------ */
/* View models                                                         */
/* ------------------------------------------------------------------ */

// invoiceForm is the invoice as the form displays it: everything as text,
// exactly as it will appear in the inputs.
type invoiceForm struct {
	Number              string
	ClientID            int64
	ProjectName         string
	ContractRef         string
	PONumber            string
	InvoiceDate         string
	PeriodStart         string
	PeriodEnd           string
	TermsDays           string
	Expenses            string
	Discount            string
	PaymentInstructions string
	Notes               string
	FooterMessage       string
}

type formItem struct {
	WorkDate    string
	Description string
	Detail      string
	Hours       string
	Rate        string
}

// clientOption feeds the browser script that applies a client's defaults when
// the client dropdown changes.
type clientOption struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	DefaultProject   string `json:"default_project"`
	DefaultContract  string `json:"default_contract"`
	DefaultPO        string `json:"default_po"`
	DefaultRate      string `json:"default_rate"`
	DefaultTermsDays *int64 `json:"default_terms_days"`
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}

// formatHoursInput renders stored hours for an input: 8 → "8", 7.5 → "7.5",
// and 0 → "" so untouched rows stay blank.
func formatHoursInput(hours float64) string {
	if hours == 0 {
		return ""
	}
	return strconv.FormatFloat(hours, 'f', -1, 64)
}

func toFormItem(item store.LineItem) formItem {
	return formItem{
		WorkDate:    item.WorkDate,
		Description: item.Description,
		Detail:      item.Detail,
		Hours:       formatHoursInput(item.Hours),
		Rate:        money.CentsToInput(item.RateCents),
	}
}

func toFormItems(items []store.LineItem) []formItem {
	out := make([]formItem, len(items))
	for i, item := range items {
		out[i] = toFormItem(item)
	}
	return out
}

func blankWeekItems(startISO string, rateCents int64) []formItem {
	var items []formItem
	for _, date := range dates.WeekdaysFrom(startISO) {
		items = append(items, formItem{WorkDate: date, Rate: money.CentsToInput(rateCents)})
	}
	return items
}

// submittedForm redisplays a rejected submission: the validated values where
// there are some, and the raw expenses and discount as typed.
func submittedForm(values store.InvoiceData, form url.Values) invoiceForm {
	return invoiceForm{
		Number:              values.Number,
		ClientID:            values.ClientID,
		ProjectName:         values.ProjectName,
		ContractRef:         values.ContractRef,
		PONumber:            values.PONumber,
		InvoiceDate:         values.InvoiceDate,
		PeriodStart:         values.PeriodStart,
		PeriodEnd:           values.PeriodEnd,
		TermsDays:           itoa(values.TermsDays),
		Expenses:            form.Get("expenses"),
		Discount:            form.Get("discount"),
		PaymentInstructions: values.PaymentInstructions,
		Notes:               values.Notes,
		FooterMessage:       values.FooterMessage,
	}
}

type formParams struct {
	mode      string // "new" or "edit"
	action    string
	invoiceID int64
	invoice   invoiceForm
	items     []formItem
	errors    []string
}

// renderForm renders the invoice form, for both new and edit.
func (s *server) renderForm(c *gin.Context, status int, p formParams) error {
	clients, err := s.store.ListClients(false)
	if err != nil {
		return err
	}
	nextNumber, err := s.store.NextInvoiceNumber(time.Now().Year())
	if err != nil {
		return err
	}

	options := make([]clientOption, len(clients))
	for i, client := range clients {
		rate := ""
		if client.DefaultRateCents != nil {
			rate = money.CentsToInput(*client.DefaultRateCents)
		}
		options[i] = clientOption{
			ID:               client.ID,
			Name:             client.Name,
			DefaultProject:   client.DefaultProject,
			DefaultContract:  client.DefaultContract,
			DefaultPO:        client.DefaultPO,
			DefaultRate:      rate,
			DefaultTermsDays: client.DefaultTermsDays,
		}
	}

	title := "New invoice"
	if p.mode == "edit" {
		title = "Edit invoice " + p.invoice.Number
	}

	return s.pages.render(c, status, "invoices/form", gin.H{
		"Title":         title,
		"Mode":          p.mode,
		"Action":        p.action,
		"InvoiceID":     p.invoiceID,
		"Invoice":       p.invoice,
		"Items":         p.items,
		"Errors":        p.errors,
		"Clients":       clients,
		"ClientOptions": options,
		"NextNumber":    nextNumber,
		"HasClients":    len(clients) > 0,
	})
}

// clientDefaults picks the rate and terms for a new invoice: the client's own
// if it has them, otherwise the ones from Settings.
func clientDefaults(client *store.Client, settings store.Settings) (rateCents, termsDays int64) {
	rateCents, termsDays = settings.DefaultRateCents, settings.DefaultTermsDays
	if client != nil && client.DefaultRateCents != nil {
		rateCents = *client.DefaultRateCents
	}
	if client != nil && client.DefaultTermsDays != nil {
		termsDays = *client.DefaultTermsDays
	}
	return rateCents, termsDays
}

// lookupInvoice finds the invoice named by a raw id, or returns a 404.
func (s *server) lookupInvoice(raw string) (*store.Invoice, error) {
	var invoice *store.Invoice
	if id, ok := parseID(raw); ok {
		var err error
		if invoice, err = s.store.Invoice(id); err != nil {
			return nil, err
		}
	}
	if invoice == nil {
		return nil, notFound("No invoice with id %s.", raw)
	}
	return invoice, nil
}

// lookupClient finds the client named by a raw id, or nil if there is none.
func (s *server) lookupClient(raw string) (*store.Client, error) {
	id, ok := parseID(raw)
	if !ok {
		return nil, nil
	}
	return s.store.Client(id)
}

/* ------------------------------------------------------------------ */
/* List                                                                */
/* ------------------------------------------------------------------ */

func (s *server) listInvoices(c *gin.Context) error {
	invoices, err := s.store.ListInvoices()
	if err != nil {
		return err
	}
	stats, err := s.store.InvoiceStats()
	if err != nil {
		return err
	}
	clients, err := s.store.ListClients(false)
	if err != nil {
		return err
	}

	return s.pages.render(c, http.StatusOK, "invoices/list", gin.H{
		"Title":      "Invoices",
		"Invoices":   invoices,
		"Stats":      stats,
		"HasClients": len(clients) > 0,
	})
}

/* ------------------------------------------------------------------ */
/* New                                                                 */
/* ------------------------------------------------------------------ */

func (s *server) newInvoice(c *gin.Context) error {
	settings, err := s.store.Settings()
	if err != nil {
		return err
	}
	clients, err := s.store.ListClients(false)
	if err != nil {
		return err
	}

	if copyFrom := c.Query("copy_from"); copyFrom != "" {
		return s.repeatInvoice(c, copyFrom, settings)
	}

	var client *store.Client
	if raw := c.Query("client_id"); raw != "" {
		if client, err = s.lookupClient(raw); err != nil {
			return err
		}
	} else if len(clients) > 0 {
		client = &clients[0]
	}

	rateCents, termsDays := clientDefaults(client, settings)
	week := c.Query("week")
	if !dates.IsISO(week) {
		week = dates.Today()
	}
	start, end := dates.WorkWeekOf(week)

	invoice := invoiceForm{
		InvoiceDate:         dates.Today(),
		PeriodStart:         start,
		PeriodEnd:           end,
		TermsDays:           itoa(termsDays),
		Expenses:            "0.00",
		Discount:            "0.00",
		PaymentInstructions: settings.PaymentInstructions,
		Notes:               settings.Notes,
		FooterMessage:       settings.FooterMessage,
	}
	if client != nil {
		invoice.ClientID = client.ID
		invoice.ProjectName = client.DefaultProject
		invoice.ContractRef = client.DefaultContract
		invoice.PONumber = client.DefaultPO
	}

	return s.renderForm(c, http.StatusOK, formParams{
		mode:    "new",
		action:  "/invoices",
		invoice: invoice,
		items:   blankWeekItems(start, rateCents),
	})
}

// repeatInvoice rolls a whole invoice forward one week, keeping the work
// descriptions as a starting point for the new period.
func (s *server) repeatInvoice(c *gin.Context, copyFrom string, settings store.Settings) error {
	source, err := s.lookupInvoice(copyFrom)
	if err != nil {
		return err
	}
	sourceItems, err := s.store.LineItems(source.ID)
	if err != nil {
		return err
	}

	items := make([]formItem, len(sourceItems))
	for i, item := range sourceItems {
		items[i] = toFormItem(item)
		if item.WorkDate != "" {
			items[i].WorkDate = dates.AddDays(item.WorkDate, 7)
		}
		items[i].Hours = ""
	}
	if len(items) == 0 {
		items = blankWeekItems(dates.Today(), settings.DefaultRateCents)
	}

	invoice := invoiceForm{
		ClientID:            source.ClientID,
		ProjectName:         source.ProjectName,
		ContractRef:         source.ContractRef,
		PONumber:            source.PONumber,
		InvoiceDate:         dates.Today(),
		TermsDays:           itoa(source.TermsDays),
		Expenses:            "0.00",
		Discount:            "0.00",
		PaymentInstructions: source.PaymentInstructions,
		Notes:               source.Notes,
		FooterMessage:       source.FooterMessage,
	}
	if source.PeriodStart != "" {
		invoice.PeriodStart = dates.AddDays(source.PeriodStart, 7)
	}
	if source.PeriodEnd != "" {
		invoice.PeriodEnd = dates.AddDays(source.PeriodEnd, 7)
	}

	return s.renderForm(c, http.StatusOK, formParams{
		mode:    "new",
		action:  "/invoices",
		invoice: invoice,
		items:   items,
	})
}

/* ------------------------------------------------------------------ */
/* Create                                                              */
/* ------------------------------------------------------------------ */

func (s *server) createInvoice(c *gin.Context) error {
	form := c.Request.PostForm
	settings, err := s.store.Settings()
	if err != nil {
		return err
	}

	var client *store.Client
	if raw := form.Get("client_id"); raw != "" {
		if client, err = s.lookupClient(raw); err != nil {
			return err
		}
	}

	values, items, errs := forms.BuildInvoice(form, forms.BusinessSnapshot(settings), forms.ClientSnapshot(client))

	if values.Number == "" {
		if values.Number, err = s.store.NextInvoiceNumber(time.Now().Year()); err != nil {
			return err
		}
	}
	exists, err := s.store.NumberExists(values.Number, 0)
	if err != nil {
		return err
	}
	if exists {
		errs = append(errs, "Invoice number "+values.Number+" is already in use.")
	}

	if len(errs) > 0 {
		return s.renderForm(c, http.StatusUnprocessableEntity, formParams{
			mode:    "new",
			action:  "/invoices",
			invoice: submittedForm(values, form),
			items:   toFormItems(items),
			errors:  errs,
		})
	}

	id, err := s.store.CreateInvoice(values, items)
	if err != nil {
		return err
	}
	c.Redirect(http.StatusFound, "/invoices/"+itoa(id))
	return nil
}

/* ------------------------------------------------------------------ */
/* Edit / Update / Delete                                              */
/* ------------------------------------------------------------------ */

func (s *server) editInvoice(c *gin.Context) error {
	invoice, err := s.lookupInvoice(c.Param("id"))
	if err != nil {
		return err
	}
	items, err := s.store.LineItems(invoice.ID)
	if err != nil {
		return err
	}

	return s.renderForm(c, http.StatusOK, formParams{
		mode:      "edit",
		action:    "/invoices/" + itoa(invoice.ID),
		invoiceID: invoice.ID,
		invoice: invoiceForm{
			Number:              invoice.Number,
			ClientID:            invoice.ClientID,
			ProjectName:         invoice.ProjectName,
			ContractRef:         invoice.ContractRef,
			PONumber:            invoice.PONumber,
			InvoiceDate:         invoice.InvoiceDate,
			PeriodStart:         invoice.PeriodStart,
			PeriodEnd:           invoice.PeriodEnd,
			TermsDays:           itoa(invoice.TermsDays),
			Expenses:            money.CentsToInput(invoice.ExpensesCents),
			Discount:            money.CentsToInput(invoice.DiscountCents),
			PaymentInstructions: invoice.PaymentInstructions,
			Notes:               invoice.Notes,
			FooterMessage:       invoice.FooterMessage,
		},
		items: toFormItems(items),
	})
}

func (s *server) updateInvoice(c *gin.Context) error {
	existing, err := s.lookupInvoice(c.Param("id"))
	if err != nil {
		return err
	}
	form := c.Request.PostForm

	// The business block stays frozen at whatever it was when the invoice was
	// generated. The client block is only re-snapshotted if you switch clients.
	client := existing.ClientSnapshot
	if submitted, ok := parseID(form.Get("client_id")); ok && submitted != 0 && submitted != existing.ClientID {
		newClient, err := s.store.Client(submitted)
		if err != nil {
			return err
		}
		client = forms.ClientSnapshot(newClient)
	}

	values, items, errs := forms.BuildInvoice(form, existing.BusinessSnapshot, client)

	if values.Number == "" {
		values.Number = existing.Number
	}
	exists, err := s.store.NumberExists(values.Number, existing.ID)
	if err != nil {
		return err
	}
	if exists {
		errs = append(errs, "Invoice number "+values.Number+" is already in use.")
	}

	if len(errs) > 0 {
		return s.renderForm(c, http.StatusUnprocessableEntity, formParams{
			mode:      "edit",
			action:    "/invoices/" + itoa(existing.ID),
			invoiceID: existing.ID,
			invoice:   submittedForm(values, form),
			items:     toFormItems(items),
			errors:    errs,
		})
	}

	if err := s.store.UpdateInvoice(existing.ID, values, items); err != nil {
		return err
	}
	c.Redirect(http.StatusFound, "/invoices/"+itoa(existing.ID))
	return nil
}

func (s *server) deleteInvoice(c *gin.Context) error {
	if id, ok := parseID(c.Param("id")); ok {
		if err := s.store.DeleteInvoice(id); err != nil {
			return err
		}
	}
	c.Redirect(http.StatusFound, "/")
	return nil
}

/* ------------------------------------------------------------------ */
/* Print view                                                          */
/* ------------------------------------------------------------------ */

func (s *server) printInvoice(c *gin.Context) error {
	invoice, err := s.lookupInvoice(c.Param("id"))
	if err != nil {
		return err
	}
	items, err := s.store.LineItems(invoice.ID)
	if err != nil {
		return err
	}

	lines := make([]money.Line, len(items))
	for i, item := range items {
		lines[i] = money.Line{Hours: item.Hours, RateCents: item.RateCents}
	}

	return s.pages.render(c, http.StatusOK, "invoices/print", gin.H{
		"Title":   "Invoice " + invoice.Number,
		"Invoice": invoice,
		"Items":   items,
		"Totals":  money.ComputeTotals(lines, invoice.ExpensesCents, invoice.DiscountCents),
	})
}
