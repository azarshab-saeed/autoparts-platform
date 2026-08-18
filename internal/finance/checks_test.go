package finance

import "testing"

func TestParseCheckDateAcceptsPersianDate(t *testing.T) {
	got, err := parseCheckDate("۱۴۰۵/۰۵/۲۷", "due_date")
	if err != nil {
		t.Fatal(err)
	}
	if got.Format("2006-01-02") != "2026-08-18" {
		t.Fatalf("unexpected Gregorian conversion: %s", got.Format("2006-01-02"))
	}
}

func TestParseCheckDateRejectsInvalidPersianDate(t *testing.T) {
	if _, err := parseCheckDate("1405/13/01", "due_date"); err == nil {
		t.Fatal("expected invalid Persian date")
	}
}
