package store

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"invoices/internal/dates"
	"invoices/internal/jscompat"
)

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func nullableInt(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func fromNull(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

/* ------------------------------------------------------------------ */
/* Settings                                                            */
/* ------------------------------------------------------------------ */

const settingsColumns = `business_name, business_tagline, address_line1, address_line2,
  city_state_zip, email, phone, tax_id, default_rate_cents, default_terms_days,
  payment_instructions, notes, footer_message`

func (s *Store) Settings() (Settings, error) {
	var st Settings
	err := s.db.QueryRow(`SELECT `+settingsColumns+` FROM settings WHERE id = 1`).Scan(
		&st.BusinessName, &st.BusinessTagline, &st.AddressLine1, &st.AddressLine2,
		&st.CityStateZip, &st.Email, &st.Phone, &st.TaxID, &st.DefaultRateCents,
		&st.DefaultTermsDays, &st.PaymentInstructions, &st.Notes, &st.FooterMessage,
	)
	return st, err
}

func (s *Store) SaveSettings(st Settings) error {
	_, err := s.db.Exec(
		`UPDATE settings SET
		   business_name = ?, business_tagline = ?, address_line1 = ?, address_line2 = ?,
		   city_state_zip = ?, email = ?, phone = ?, tax_id = ?, default_rate_cents = ?,
		   default_terms_days = ?, payment_instructions = ?, notes = ?, footer_message = ?
		 WHERE id = 1`,
		st.BusinessName, st.BusinessTagline, st.AddressLine1, st.AddressLine2,
		st.CityStateZip, st.Email, st.Phone, st.TaxID, st.DefaultRateCents,
		st.DefaultTermsDays, st.PaymentInstructions, st.Notes, st.FooterMessage,
	)
	return err
}

/* ------------------------------------------------------------------ */
/* Clients                                                             */
/* ------------------------------------------------------------------ */

const clientColumns = `c.id, c.name, c.attn, c.address_line1, c.address_line2, c.city_state_zip,
  c.email, c.default_project, c.default_contract, c.default_po, c.default_rate_cents,
  c.default_terms_days, c.archived, c.created_at`

func scanClient(row rowScanner, extra ...any) (Client, error) {
	var c Client
	var rate, terms sql.NullInt64
	var archived int64
	dest := []any{
		&c.ID, &c.Name, &c.Attn, &c.AddressLine1, &c.AddressLine2, &c.CityStateZip,
		&c.Email, &c.DefaultProject, &c.DefaultContract, &c.DefaultPO, &rate,
		&terms, &archived, &c.CreatedAt,
	}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return c, err
	}
	c.DefaultRateCents = fromNull(rate)
	c.DefaultTermsDays = fromNull(terms)
	c.Archived = archived != 0
	return c, nil
}

// ListClients returns clients by name, with their invoice counts. Archived
// clients are only included when asked for, and then sort last.
func (s *Store) ListClients(includeArchived bool) ([]Client, error) {
	query := `SELECT ` + clientColumns + `,
	            (SELECT COUNT(*) FROM invoices WHERE client_id = c.id) AS invoice_count
	          FROM clients c `
	if includeArchived {
		query += `ORDER BY c.archived, c.name COLLATE NOCASE`
	} else {
		query += `WHERE c.archived = 0 ORDER BY c.name COLLATE NOCASE`
	}

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []Client
	for rows.Next() {
		var count int64
		c, err := scanClient(rows, &count)
		if err != nil {
			return nil, err
		}
		c.InvoiceCount = count
		clients = append(clients, c)
	}
	return clients, rows.Err()
}

// Client returns the client with id, or nil if there is none.
func (s *Store) Client(id int64) (*Client, error) {
	c, err := scanClient(s.db.QueryRow(`SELECT `+clientColumns+` FROM clients c WHERE c.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func clientArgs(c ClientData) []any {
	return []any{
		c.Name, c.Attn, c.AddressLine1, c.AddressLine2, c.CityStateZip, c.Email,
		c.DefaultProject, c.DefaultContract, c.DefaultPO,
		nullableInt(c.DefaultRateCents), nullableInt(c.DefaultTermsDays), boolInt(c.Archived),
	}
}

func (s *Store) CreateClient(c ClientData) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO clients (
		   name, attn, address_line1, address_line2, city_state_zip, email,
		   default_project, default_contract, default_po,
		   default_rate_cents, default_terms_days, archived, created_at
		 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		append(clientArgs(c), dates.Now())...,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Store) UpdateClient(id int64, c ClientData) error {
	_, err := s.db.Exec(
		`UPDATE clients SET
		   name = ?, attn = ?, address_line1 = ?, address_line2 = ?, city_state_zip = ?, email = ?,
		   default_project = ?, default_contract = ?, default_po = ?,
		   default_rate_cents = ?, default_terms_days = ?, archived = ?
		 WHERE id = ?`,
		append(clientArgs(c), id)...,
	)
	return err
}

// DeleteClient removes a client. Its invoices keep their snapshotted billing
// details; the foreign key is set to NULL.
func (s *Store) DeleteClient(id int64) error {
	_, err := s.db.Exec(`DELETE FROM clients WHERE id = ?`, id)
	return err
}

/* ------------------------------------------------------------------ */
/* Invoices                                                            */
/* ------------------------------------------------------------------ */

const invoiceDataColumns = `number, client_id,
  business_name, business_tagline, business_address_line1, business_address_line2,
  business_city_state_zip, business_email, business_phone, business_tax_id,
  client_name, client_attn, client_address_line1, client_address_line2,
  client_city_state_zip, client_email,
  project_name, contract_ref, po_number,
  invoice_date, period_start, period_end, terms_days, due_date,
  expenses_cents, discount_cents,
  payment_instructions, notes, footer_message`

const invoiceSummarySQL = `
  SELECT
    i.id, ` + invoiceDataColumns + `, i.generated_at, i.updated_at,
    COALESCE((SELECT SUM(amount_cents) FROM line_items WHERE invoice_id = i.id), 0) AS subtotal_cents,
    COALESCE((SELECT SUM(hours)        FROM line_items WHERE invoice_id = i.id), 0) AS total_hours,
    COALESCE((SELECT SUM(amount_cents) FROM line_items WHERE invoice_id = i.id), 0)
      + i.expenses_cents - i.discount_cents AS total_cents
  FROM invoices i
`

func invoiceDataArgs(d InvoiceData) []any {
	var clientID any
	if d.ClientID != 0 {
		clientID = d.ClientID
	}
	return []any{
		d.Number, clientID,
		d.BusinessName, d.BusinessTagline, d.BusinessAddressLine1, d.BusinessAddressLine2,
		d.BusinessCityStateZip, d.BusinessEmail, d.BusinessPhone, d.BusinessTaxID,
		d.ClientName, d.ClientAttn, d.ClientAddressLine1, d.ClientAddressLine2,
		d.ClientCityStateZip, d.ClientEmail,
		d.ProjectName, d.ContractRef, d.PONumber,
		d.InvoiceDate, d.PeriodStart, d.PeriodEnd, d.TermsDays, d.DueDate,
		d.ExpensesCents, d.DiscountCents,
		d.PaymentInstructions, d.Notes, d.FooterMessage,
	}
}

func scanInvoice(row rowScanner) (Invoice, error) {
	var inv Invoice
	var clientID sql.NullInt64
	err := row.Scan(
		&inv.ID, &inv.Number, &clientID,
		&inv.BusinessName, &inv.BusinessTagline, &inv.BusinessAddressLine1, &inv.BusinessAddressLine2,
		&inv.BusinessCityStateZip, &inv.BusinessEmail, &inv.BusinessPhone, &inv.BusinessTaxID,
		&inv.ClientName, &inv.ClientAttn, &inv.ClientAddressLine1, &inv.ClientAddressLine2,
		&inv.ClientCityStateZip, &inv.ClientEmail,
		&inv.ProjectName, &inv.ContractRef, &inv.PONumber,
		&inv.InvoiceDate, &inv.PeriodStart, &inv.PeriodEnd, &inv.TermsDays, &inv.DueDate,
		&inv.ExpensesCents, &inv.DiscountCents,
		&inv.PaymentInstructions, &inv.Notes, &inv.FooterMessage,
		&inv.GeneratedAt, &inv.UpdatedAt,
		&inv.SubtotalCents, &inv.TotalHours, &inv.TotalCents,
	)
	inv.ClientID = clientID.Int64
	return inv, err
}

func (s *Store) ListInvoices() ([]Invoice, error) {
	rows, err := s.db.Query(invoiceSummarySQL + ` ORDER BY i.invoice_date DESC, i.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invoices []Invoice
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			return nil, err
		}
		invoices = append(invoices, inv)
	}
	return invoices, rows.Err()
}

// Invoice returns the invoice with id, or nil if there is none.
func (s *Store) Invoice(id int64) (*Invoice, error) {
	inv, err := scanInvoice(s.db.QueryRow(invoiceSummarySQL+` WHERE i.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (s *Store) LineItems(invoiceID int64) ([]LineItem, error) {
	rows, err := s.db.Query(
		`SELECT id, invoice_id, position, work_date, description, detail, hours, rate_cents, amount_cents
		   FROM line_items WHERE invoice_id = ? ORDER BY position, id`,
		invoiceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []LineItem
	for rows.Next() {
		var it LineItem
		if err := rows.Scan(&it.ID, &it.InvoiceID, &it.Position, &it.WorkDate, &it.Description,
			&it.Detail, &it.Hours, &it.RateCents, &it.AmountCents); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *Store) InvoiceStats() (InvoiceStats, error) {
	var st InvoiceStats
	err := s.db.QueryRow(
		`SELECT
		   COUNT(*),
		   COALESCE(SUM(total_cents), 0),
		   COALESCE(SUM(total_hours), 0)
		 FROM (`+invoiceSummarySQL+`)`,
	).Scan(&st.Count, &st.TotalCents, &st.TotalHours)
	return st, err
}

// NextInvoiceNumber returns the next sequential number for a year, e.g.
// "2026-004".
func (s *Store) NextInvoiceNumber(year int) (string, error) {
	prefix := strconv.Itoa(year)
	var number string
	err := s.db.QueryRow(
		`SELECT number FROM invoices
		  WHERE number LIKE ?
		  ORDER BY CAST(substr(number, 6) AS INTEGER) DESC
		  LIMIT 1`,
		prefix+"-%",
	).Scan(&number)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	var previous int64
	if len(number) > 5 {
		previous, _ = jscompat.ParseInt(number[5:])
	}
	next := strconv.FormatInt(previous+1, 10)
	if len(next) < 3 {
		next = strings.Repeat("0", 3-len(next)) + next
	}
	return prefix + "-" + next, nil
}

// NumberExists reports whether another invoice (other than exceptID, if
// non-zero) already uses number.
func (s *Store) NumberExists(number string, exceptID int64) (bool, error) {
	var id int64
	var err error
	if exceptID != 0 {
		err = s.db.QueryRow(`SELECT id FROM invoices WHERE number = ? AND id != ?`, number, exceptID).Scan(&id)
	} else {
		err = s.db.QueryRow(`SELECT id FROM invoices WHERE number = ?`, number).Scan(&id)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func insertItems(tx *sql.Tx, invoiceID int64, items []LineItem) error {
	stmt, err := tx.Prepare(
		`INSERT INTO line_items (invoice_id, position, work_date, description, detail, hours, rate_cents)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for index, item := range items {
		if _, err := stmt.Exec(invoiceID, index, item.WorkDate, item.Description, item.Detail,
			item.Hours, item.RateCents); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateInvoice(data InvoiceData, items []LineItem) (int64, error) {
	var id int64
	err := s.transaction(func(tx *sql.Tx) error {
		now := dates.Now()
		result, err := tx.Exec(
			`INSERT INTO invoices (`+invoiceDataColumns+`, generated_at, updated_at)
			 VALUES (`+placeholders(29)+`, ?, ?)`,
			append(invoiceDataArgs(data), now, now)...,
		)
		if err != nil {
			return err
		}
		if id, err = result.LastInsertId(); err != nil {
			return err
		}
		return insertItems(tx, id, items)
	})
	return id, err
}

// UpdateInvoice rewrites an invoice and bumps updated_at. generated_at is
// never touched.
func (s *Store) UpdateInvoice(id int64, data InvoiceData, items []LineItem) error {
	return s.transaction(func(tx *sql.Tx) error {
		assignments := strings.Split(invoiceDataColumns, ",")
		for i, column := range assignments {
			assignments[i] = strings.TrimSpace(column) + " = ?"
		}

		args := append(invoiceDataArgs(data), dates.Now(), id)
		if _, err := tx.Exec(
			`UPDATE invoices SET `+strings.Join(assignments, ", ")+`, updated_at = ? WHERE id = ?`,
			args...,
		); err != nil {
			return err
		}

		// Items are fully replaced: simpler and safer than diffing, and the
		// row counts here are tiny.
		if _, err := tx.Exec(`DELETE FROM line_items WHERE invoice_id = ?`, id); err != nil {
			return err
		}
		return insertItems(tx, id, items)
	})
}

func (s *Store) DeleteInvoice(id int64) error {
	_, err := s.db.Exec(`DELETE FROM invoices WHERE id = ?`, id)
	return err
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}
