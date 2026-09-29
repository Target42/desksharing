package auth

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"
)

const (
	maxImportBytes = 2 << 20
	maxImportRows  = 500
)

type ImportRow struct {
	Row         int    `json:"row"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name,omitempty"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
	InviteToken string `json:"invite_token,omitempty"`
}

type ImportMembersResult struct {
	Created int         `json:"created"`
	Skipped int         `json:"skipped"`
	Failed  int         `json:"failed"`
	Rows    []ImportRow `json:"rows"`
}

type parsedMember struct {
	Row         int
	Email       string
	DisplayName string
}

func (s *Service) ImportMembers(ctx context.Context, tenantID uuid.UUID, filename string, r io.Reader) (*ImportMembersResult, error) {
	if err := s.requireActiveTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	parsed, err := ParseMemberFile(filename, r)
	if err != nil {
		return nil, err
	}

	out := &ImportMembersResult{Rows: []ImportRow{}}
	seen := map[string]struct{}{}
	for _, row := range parsed {
		email := normalizeEmail(row.Email)
		name := strings.TrimSpace(row.DisplayName)
		item := ImportRow{Row: row.Row, Email: email, DisplayName: name}

		if !isPlausibleEmail(email) {
			item.Status = "error"
			item.Message = "E-Mail ist ungültig"
			out.Failed++
			out.Rows = append(out.Rows, item)
			continue
		}
		if _, ok := seen[email]; ok {
			item.Status = "skipped"
			item.Message = "E-Mail kommt in der Datei mehrfach vor"
			out.Skipped++
			out.Rows = append(out.Rows, item)
			continue
		}
		seen[email] = struct{}{}
		if name == "" {
			name = nameFromEmail(email)
			item.DisplayName = name
		}
		if len(name) > 200 {
			item.Status = "error"
			item.Message = "Name ist zu lang"
			out.Failed++
			out.Rows = append(out.Rows, item)
			continue
		}

		member, rawToken, err := s.inviteLocalMember(ctx, tenantID, email, name)
		if err != nil {
			if e, ok := AsError(err); ok && (e.Code == "conflict") {
				item.Status = "skipped"
				item.Message = e.Message
				out.Skipped++
				out.Rows = append(out.Rows, item)
				continue
			}
			if e, ok := AsError(err); ok {
				item.Status = "error"
				item.Message = e.Message
				out.Failed++
				out.Rows = append(out.Rows, item)
				continue
			}
			return nil, err
		}
		s.sendInvite(ctx, member.Email, member.DisplayName, rawToken)
		item.Status = "created"
		item.DisplayName = member.DisplayName
		item.Message = "Einladung verschickt"
		if s.devReturnTok {
			item.InviteToken = rawToken
		}
		out.Created++
		out.Rows = append(out.Rows, item)
	}
	return out, nil
}

func ParseMemberFile(filename string, r io.Reader) ([]parsedMember, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxImportBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read import: %w", err)
	}
	if len(data) > maxImportBytes {
		return nil, ErrInvalidInput("Die Datei ist zu groß (max. 2 MB)")
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, ErrInvalidInput("Die Datei ist leer")
	}

	ext := strings.ToLower(filepath.Ext(filename))
	var table [][]string
	switch ext {
	case ".csv", ".txt", "":
		table, err = readCSVTable(data)
	case ".xlsx":
		table, err = readXLSXTable(data)
	case ".xls":
		return nil, ErrInvalidInput("Alte Excel-Dateien (.xls) werden nicht unterstützt. Bitte als .xlsx oder CSV speichern.")
	default:
		return nil, ErrInvalidInput("Unbekanntes Format. Bitte CSV oder Excel (.xlsx) verwenden.")
	}
	if err != nil {
		return nil, err
	}
	return membersFromTable(table)
}

func MemberImportTemplateCSV() []byte {
	return []byte("\uFEFFE-Mail;Name\r\n")
}

func MemberImportTemplateXLSX() ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)
	_ = f.SetCellValue(sheet, "A1", "E-Mail")
	_ = f.SetCellValue(sheet, "B1", "Name")
	_ = f.SetColWidth(sheet, "A", "A", 36)
	_ = f.SetColWidth(sheet, "B", "B", 28)
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("template xlsx: %w", err)
	}
	return buf.Bytes(), nil
}

func readCSVTable(data []byte) ([][]string, error) {
	text := decodeImportText(data)
	text = strings.TrimPrefix(text, "\uFEFF")
	sample := text
	if i := strings.IndexAny(sample, "\r\n"); i >= 0 {
		sample = sample[:i]
	}
	reader := csv.NewReader(strings.NewReader(text))
	reader.Comma = detectDelimiter(sample)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, ErrInvalidInput("CSV konnte nicht gelesen werden")
	}
	return rows, nil
}

func readXLSXTable(data []byte) ([][]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, ErrInvalidInput("Die Excel-Datei konnte nicht gelesen werden")
	}
	defer f.Close()
	sheet := f.GetSheetName(0)
	if sheet == "" {
		return nil, ErrInvalidInput("Die Excel-Datei enthält kein Blatt")
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, ErrInvalidInput("Die Excel-Datei konnte nicht gelesen werden")
	}
	return rows, nil
}

func membersFromTable(table [][]string) ([]parsedMember, error) {
	start := 0
	cols := colMap{email: -1, name: -1, first: -1, last: -1}
	headerless := false
	for i, row := range table {
		if rowEmpty(row) {
			continue
		}
		mapped, ok := mapColumns(row)
		if ok {
			cols = mapped
			start = i + 1
			break
		}
		if isPlausibleEmail(normalizeEmail(cell(row, 0))) {
			cols = colMap{email: 0, name: 1, first: -1, last: -1}
			start = i
			headerless = true
			break
		}
		return nil, ErrInvalidInput("Spalte E-Mail fehlt. Erwartet werden die Spalten E-Mail und Name.")
	}
	if !headerless && cols.email < 0 {
		return nil, ErrInvalidInput("Spalte E-Mail fehlt. Erwartet werden die Spalten E-Mail und Name.")
	}

	out := make([]parsedMember, 0, len(table))
	for i := start; i < len(table); i++ {
		row := table[i]
		email := normalizeEmail(cell(row, cols.email))
		name := cols.displayName(row)
		if email == "" && name == "" {
			continue
		}
		out = append(out, parsedMember{
			Row:         i + 1,
			Email:       email,
			DisplayName: name,
		})
		if len(out) > maxImportRows {
			return nil, ErrInvalidInput(fmt.Sprintf("Höchstens %d Mitarbeiter pro Datei", maxImportRows))
		}
	}
	if len(out) == 0 {
		return nil, ErrInvalidInput("Die Datei enthält keine Mitarbeiter")
	}
	return out, nil
}

type colMap struct {
	email int
	name  int
	first int
	last  int
}

func mapColumns(headers []string) (colMap, bool) {
	cols := colMap{email: -1, name: -1, first: -1, last: -1}
	for i, h := range headers {
		switch normHeader(h) {
		case "email", "mail":
			cols.email = i
		case "name", "anzeigename", "displayname", "mitarbeiter", "vollname", "mitarbeitername":
			if cols.name < 0 {
				cols.name = i
			}
		case "vorname":
			cols.first = i
		case "nachname":
			cols.last = i
		}
	}
	return cols, cols.email >= 0
}

func (c colMap) displayName(row []string) string {
	if c.name >= 0 {
		if v := strings.TrimSpace(cell(row, c.name)); v != "" {
			return v
		}
	}
	first := ""
	last := ""
	if c.first >= 0 {
		first = strings.TrimSpace(cell(row, c.first))
	}
	if c.last >= 0 {
		last = strings.TrimSpace(cell(row, c.last))
	}
	return strings.TrimSpace(first + " " + last)
}

func cell(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func rowEmpty(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func detectDelimiter(header string) rune {
	if strings.Count(header, ";") > strings.Count(header, ",") {
		return ';'
	}
	return ','
}

func normHeader(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(s, "\uFEFF"))
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	return s
}

func decodeImportText(data []byte) string {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if utf8.Valid(data) {
		return string(data)
	}
	decoded, err := charmap.Windows1252.NewDecoder().Bytes(data)
	if err != nil {
		return string(data)
	}
	return string(decoded)
}

func isPlausibleEmail(email string) bool {
	if email == "" || len(email) > 254 || strings.ContainsAny(email, " \t") {
		return false
	}
	at := strings.Index(email, "@")
	if at <= 0 || at != strings.LastIndex(email, "@") || at >= len(email)-1 {
		return false
	}
	domain := email[at+1:]
	dot := strings.LastIndex(domain, ".")
	return dot > 0 && dot < len(domain)-1
}

func nameFromEmail(email string) string {
	local := email
	if i := strings.Index(email, "@"); i > 0 {
		local = email[:i]
	}
	local = strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(local)
	return strings.TrimSpace(local)
}
