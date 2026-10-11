package annxlsx

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gongkao/internal/model"

	"github.com/xuri/excelize/v2"
)

const sheetName = "announcements"

const timeLayout = "2006-01-02 15:04"

// Columns is the announcement workbook header, in order.
var Columns = []string{
	"type",
	"title",
	"region",
	"source",
	"source_id",
	"source_url",
	"summary",
	"content",
	"register_start",
	"register_end",
	"confirm_start",
	"confirm_end",
	"exam_at",
	"published_at",
	"status",
}

// Write saves announcements to an xlsx file, creating parent directories.
func Write(path string, rows []model.Announcement) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", sheetName); err != nil {
		return err
	}
	header := make([]interface{}, len(Columns))
	for i, c := range Columns {
		header[i] = c
	}
	if err := f.SetSheetRow(sheetName, "A1", &header); err != nil {
		return err
	}
	for i, a := range rows {
		cell, err := excelize.CoordinatesToCellName(1, i+2)
		if err != nil {
			return err
		}
		line := announcementRow(a)
		if err := f.SetSheetRow(sheetName, cell, &line); err != nil {
			return err
		}
	}
	return f.SaveAs(path)
}

// Read parses an announcement workbook. A missing file returns os.ErrNotExist.
func Read(path string) ([]model.Announcement, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sheet := sheetName
	if idx, err := f.GetSheetIndex(sheet); err != nil || idx < 0 {
		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return nil, nil
		}
		sheet = sheets[0]
	}
	table, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	if len(table) == 0 {
		return nil, nil
	}
	col := map[string]int{}
	for i, name := range table[0] {
		col[strings.TrimSpace(name)] = i
	}
	out := make([]model.Announcement, 0, len(table)-1)
	for _, rec := range table[1:] {
		title := cell(rec, col, "title")
		if title == "" {
			continue
		}
		a := model.Announcement{
			Type:          cell(rec, col, "type"),
			Title:         title,
			Region:        cell(rec, col, "region"),
			Source:        cell(rec, col, "source"),
			SourceID:      cell(rec, col, "source_id"),
			SourceURL:     cell(rec, col, "source_url"),
			Summary:       cell(rec, col, "summary"),
			Content:       cell(rec, col, "content"),
			RegisterStart: parseTime(cell(rec, col, "register_start")),
			RegisterEnd:   parseTime(cell(rec, col, "register_end")),
			ConfirmStart:  parseTime(cell(rec, col, "confirm_start")),
			ConfirmEnd:    parseTime(cell(rec, col, "confirm_end")),
			ExamAt:        parseTime(cell(rec, col, "exam_at")),
			Status:        cell(rec, col, "status"),
		}
		if t := parseTime(cell(rec, col, "published_at")); t != nil {
			a.PublishedAt = *t
		}
		out = append(out, a)
	}
	return out, nil
}

func announcementRow(a model.Announcement) []interface{} {
	pub := ""
	if !a.PublishedAt.IsZero() {
		pub = a.PublishedAt.Format(timeLayout)
	}
	return []interface{}{
		a.Type,
		a.Title,
		a.Region,
		a.Source,
		a.SourceID,
		a.SourceURL,
		a.Summary,
		a.Content,
		formatTime(a.RegisterStart),
		formatTime(a.RegisterEnd),
		formatTime(a.ConfirmStart),
		formatTime(a.ConfirmEnd),
		formatTime(a.ExamAt),
		pub,
		a.Status,
	}
}

func cell(rec []string, col map[string]int, name string) string {
	i, ok := col[name]
	if !ok || i >= len(rec) {
		return ""
	}
	return strings.TrimSpace(rec[i])
}

func formatTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format(timeLayout)
}

func parseTime(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	layouts := []string{timeLayout, "2006-01-02 15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return &t
		}
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil && n > 20000 && n < 80000 {
		t, err := excelize.ExcelDateToTime(n, false)
		if err == nil {
			return &t
		}
	}
	return nil
}
