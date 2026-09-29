package auth

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestParseMemberFileCSV(t *testing.T) {
	csvText := "\uFEFFE-Mail;Name\r\nanna@example.com;Anna Beispiel\r\n; \r\nbob@example.com;\r\n"
	rows, err := ParseMemberFile("team.csv", strings.NewReader(csvText))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows", len(rows))
	}
	if rows[0].Email != "anna@example.com" || rows[0].DisplayName != "Anna Beispiel" || rows[0].Row != 2 {
		t.Fatalf("row 0: %+v", rows[0])
	}
	if rows[1].Email != "bob@example.com" || rows[1].DisplayName != "" {
		t.Fatalf("row 1: %+v", rows[1])
	}
}

func TestParseMemberFileCommaAndSplitName(t *testing.T) {
	csvText := "Vorname,Nachname,E-Mail\nMax,Mustermann,max@example.com\n"
	rows, err := ParseMemberFile("team.csv", strings.NewReader(csvText))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].DisplayName != "Max Mustermann" || rows[0].Email != "max@example.com" {
		t.Fatalf("%+v", rows)
	}
}

func TestParseMemberFileHeaderless(t *testing.T) {
	rows, err := ParseMemberFile("team.txt", strings.NewReader("mia@example.com,Mia\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Email != "mia@example.com" || rows[0].DisplayName != "Mia" {
		t.Fatalf("%+v", rows)
	}
}

func TestParseMemberFileMissingEmailColumn(t *testing.T) {
	_, err := ParseMemberFile("team.csv", strings.NewReader("Name;Abteilung\nAnna;IT\n"))
	if err == nil || !strings.Contains(err.Error(), "E-Mail") {
		t.Fatalf("got %v", err)
	}
}

func TestParseMemberFileWindows1252(t *testing.T) {
	raw := []byte("E-Mail;Name\r\n")
	raw = append(raw, []byte("joerg@example.com;J")...)
	raw = append(raw, 0xF6) // ö in windows-1252
	raw = append(raw, []byte("rg\r\n")...)
	rows, err := ParseMemberFile("team.csv", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].DisplayName != "Jörg" {
		t.Fatalf("%+v", rows)
	}
}

func TestParseMemberFileXLSX(t *testing.T) {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	_ = f.SetCellValue(sheet, "A1", "Name")
	_ = f.SetCellValue(sheet, "B1", "E-Mail")
	_ = f.SetCellValue(sheet, "A2", "Sara")
	_ = f.SetCellValue(sheet, "B2", "sara@example.com")
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := ParseMemberFile("team.xlsx", bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Email != "sara@example.com" || rows[0].DisplayName != "Sara" {
		t.Fatalf("%+v", rows)
	}
}

func TestParseMemberFileRejectsXLS(t *testing.T) {
	_, err := ParseMemberFile("team.xls", strings.NewReader("E-Mail;Name\n"))
	if err == nil || !strings.Contains(err.Error(), ".xlsx") {
		t.Fatalf("got %v", err)
	}
}
