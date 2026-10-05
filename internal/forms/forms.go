// Package forms turns submitted forms into typed rows ready for SQLite, plus
// human-readable validation errors.
package forms

import (
	"net/url"

	"invoices/internal/dates"
	"invoices/internal/jscompat"
	"invoices/internal/money"
	"invoices/internal/store"
)

// Str is the trimmed first value of a form field.
func Str(form url.Values, key string) string {
	return jscompat.Trim(form.Get(key))
}

// Int parses a form-style integer the way parseInt does, falling back when
// the value is not a number.
func Int(value string, fallback int64) int64 {
	if parsed, ok := jscompat.ParseInt(jscompat.Trim(value)); ok {
		return parsed
	}
	return fallback
}

func nonNegative(n int64) int64 {
	if n < 0 {
		return 0
	}
	return n
}

// DueDate is the invoice date plus the terms, or "" without a valid date.
func DueDate(invoiceDate string, termsDays int64) string {
	if !dates.IsISO(invoiceDate) {
		return ""
	}
	return dates.AddDays(invoiceDate, int(nonNegative(termsDays)))
}

func at(values []string, i int) string {
	if i < len(values) {
		return values[i]
	}
	return ""
}

// ParseItems reads line items, which arrive as parallel arrays. Rows the user
// never touched (no description, no detail, no hours) are dropped silently.
func ParseItems(form url.Values) []store.LineItem {
	dateValues := form["item_date"]
	descriptions := form["item_description"]
	details := form["item_detail"]
	hours := form["item_hours"]
	rates := form["item_rate"]

	count := max(len(dateValues), len(descriptions), len(details), len(hours), len(rates))

	items := []store.LineItem{}
	for i := 0; i < count; i++ {
		description := jscompat.Trim(at(descriptions, i))
		detail := jscompat.Trim(at(details, i))
		parsedHours := money.ParseHours(at(hours, i))
		if description == "" && detail == "" && parsedHours == 0 {
			continue
		}

		workDate := jscompat.Trim(at(dateValues, i))
		if !dates.IsISO(workDate) {
			workDate = ""
		}
		items = append(items, store.LineItem{
			WorkDate:    workDate,
			Description: description,
			Detail:      detail,
			Hours:       parsedHours,
			RateCents:   money.ParseCents(at(rates, i)),
		})
	}
	return items
}

func BusinessSnapshot(s store.Settings) store.BusinessSnapshot {
	return store.BusinessSnapshot{
		BusinessName:         s.BusinessName,
		BusinessTagline:      s.BusinessTagline,
		BusinessAddressLine1: s.AddressLine1,
		BusinessAddressLine2: s.AddressLine2,
		BusinessCityStateZip: s.CityStateZip,
		BusinessEmail:        s.Email,
		BusinessPhone:        s.Phone,
		BusinessTaxID:        s.TaxID,
	}
}

// ClientSnapshot copies a client's billing address. A nil client yields an
// empty snapshot.
func ClientSnapshot(c *store.Client) store.ClientSnapshot {
	if c == nil {
		return store.ClientSnapshot{}
	}
	return store.ClientSnapshot{
		ClientName:         c.Name,
		ClientAttn:         c.Attn,
		ClientAddressLine1: c.AddressLine1,
		ClientAddressLine2: c.AddressLine2,
		ClientCityStateZip: c.CityStateZip,
		ClientEmail:        c.Email,
	}
}

// BuildInvoice turns a submitted invoice form into a row plus line items. The
// business and client columns are snapshotted rather than submitted with the
// form, so the caller supplies them.
func BuildInvoice(form url.Values, business store.BusinessSnapshot, client store.ClientSnapshot) (store.InvoiceData, []store.LineItem, []string) {
	errs := []string{}

	invoiceDate := Str(form, "invoice_date")
	if !dates.IsISO(invoiceDate) {
		errs = append(errs, "Invoice date is required.")
	}

	periodStart := Str(form, "period_start")
	periodEnd := Str(form, "period_end")
	if periodStart != "" && periodEnd != "" && periodEnd < periodStart {
		errs = append(errs, "The service period ends before it starts.")
	}

	clientID := Int(form.Get("client_id"), 0)
	if clientID == 0 {
		errs = append(errs, "Select a client.")
	}

	termsDays := nonNegative(Int(form.Get("terms_days"), 0))
	items := ParseItems(form)
	if len(items) == 0 {
		errs = append(errs, "Add at least one line item with a description or hours.")
	}

	if !dates.IsISO(periodStart) {
		periodStart = ""
	}
	if !dates.IsISO(periodEnd) {
		periodEnd = ""
	}

	values := store.InvoiceData{
		Number:              Str(form, "number"),
		ClientID:            clientID,
		BusinessSnapshot:    business,
		ClientSnapshot:      client,
		ProjectName:         Str(form, "project_name"),
		ContractRef:         Str(form, "contract_ref"),
		PONumber:            Str(form, "po_number"),
		InvoiceDate:         invoiceDate,
		PeriodStart:         periodStart,
		PeriodEnd:           periodEnd,
		TermsDays:           termsDays,
		DueDate:             DueDate(invoiceDate, termsDays),
		ExpensesCents:       money.ParseCents(form.Get("expenses")),
		DiscountCents:       money.ParseCents(form.Get("discount")),
		PaymentInstructions: Str(form, "payment_instructions"),
		Notes:               Str(form, "notes"),
		FooterMessage:       Str(form, "footer_message"),
	}

	return values, items, errs
}

func ParseClient(form url.Values) (store.ClientData, []string) {
	errs := []string{}
	name := Str(form, "name")
	if name == "" {
		errs = append(errs, "Client name is required.")
	}

	var rate, terms *int64
	if raw := Str(form, "default_rate"); raw != "" {
		cents := money.ParseCents(raw)
		rate = &cents
	}
	if raw := Str(form, "default_terms_days"); raw != "" {
		days := nonNegative(Int(raw, 0))
		terms = &days
	}

	return store.ClientData{
		Name:             name,
		Attn:             Str(form, "attn"),
		AddressLine1:     Str(form, "address_line1"),
		AddressLine2:     Str(form, "address_line2"),
		CityStateZip:     Str(form, "city_state_zip"),
		Email:            Str(form, "email"),
		DefaultProject:   Str(form, "default_project"),
		DefaultContract:  Str(form, "default_contract"),
		DefaultPO:        Str(form, "default_po"),
		DefaultRateCents: rate,
		DefaultTermsDays: terms,
		Archived:         form.Get("archived") != "",
	}, errs
}

func ParseSettings(form url.Values) (store.Settings, []string) {
	errs := []string{}
	name := Str(form, "business_name")
	if name == "" {
		errs = append(errs, "Business name is required.")
	}

	return store.Settings{
		BusinessName:        name,
		BusinessTagline:     Str(form, "business_tagline"),
		AddressLine1:        Str(form, "address_line1"),
		AddressLine2:        Str(form, "address_line2"),
		CityStateZip:        Str(form, "city_state_zip"),
		Email:               Str(form, "email"),
		Phone:               Str(form, "phone"),
		TaxID:               Str(form, "tax_id"),
		DefaultRateCents:    money.ParseCents(form.Get("default_rate")),
		DefaultTermsDays:    nonNegative(Int(form.Get("default_terms_days"), 15)),
		PaymentInstructions: Str(form, "payment_instructions"),
		Notes:               Str(form, "notes"),
		FooterMessage:       Str(form, "footer_message"),
	}, errs
}
