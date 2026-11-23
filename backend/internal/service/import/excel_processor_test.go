package service

import (
	"testing"

	documenttemplates "github.com/mohuganda/moh-sso-dashboard/backend/modules/document-templates"
	"github.com/xuri/excelize/v2"
)

func TestResolveWorkbookSheetNameMatchesNormalizedTemplateIdentifiers(t *testing.T) {
	workbook := excelize.NewFile()
	defer workbook.Close()

	defaultSheet := workbook.GetSheetName(0)
	if err := workbook.SetSheetName(defaultSheet, "NMS Stock Issues"); err != nil {
		t.Fatalf("rename default sheet: %v", err)
	}

	sheet := documenttemplates.TemplateSheetStructure{
		Code:        "nms_stock_issues",
		Name:        "nms_stock_issues",
		DisplayName: "NMS Stock Issues",
	}

	got := resolveWorkbookSheetName(workbook, sheet)
	if got != "NMS Stock Issues" {
		t.Fatalf("resolveWorkbookSheetName() = %q, want %q", got, "NMS Stock Issues")
	}
}

func TestResolveWorkbookSheetNamePrefersExactTemplateIdentifier(t *testing.T) {
	workbook := excelize.NewFile()
	defer workbook.Close()

	defaultSheet := workbook.GetSheetName(0)
	if err := workbook.SetSheetName(defaultSheet, "NMS Stock Issues"); err != nil {
		t.Fatalf("rename default sheet: %v", err)
	}
	if _, err := workbook.NewSheet("nms_stock_issues"); err != nil {
		t.Fatalf("create exact sheet: %v", err)
	}

	sheet := documenttemplates.TemplateSheetStructure{
		Code:        "nms_stock_issues",
		DisplayName: "NMS Stock Issues",
	}

	got := resolveWorkbookSheetName(workbook, sheet)
	if got != "nms_stock_issues" {
		t.Fatalf("resolveWorkbookSheetName() = %q, want %q", got, "nms_stock_issues")
	}
}
