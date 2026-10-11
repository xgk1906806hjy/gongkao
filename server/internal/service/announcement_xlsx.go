package service

import (
	"errors"
	"os"
	"strings"

	"gongkao/internal/annxlsx"
	"gongkao/internal/model"
)

// ImportFromXLSX loads announcements parsed from the crawl workbook.
// A missing file is not an error. After at least one row is stored, demo seed rows are taken offline.
func (s *AnnouncementService) ImportFromXLSX(path string) (int, error) {
	if strings.TrimSpace(path) == "" {
		return 0, nil
	}
	rows, err := annxlsx.Read(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	n := 0
	for i := range rows {
		a := rows[i]
		if strings.TrimSpace(a.Title) == "" || strings.TrimSpace(a.SourceURL) == "" {
			continue
		}
		if a.Source == "" {
			a.Source = "xlsx"
		}
		if a.SourceID == "" {
			a.SourceID = a.SourceURL
		}
		if a.Type == "" {
			a.Type = model.AnnouncementCivil
		}
		if a.Status == "" {
			a.Status = model.StatusPublished
		}
		if err := s.UpsertFromCrawl(&a); err != nil {
			return n, err
		}
		n++
	}
	if n > 0 {
		err := s.db.Model(&model.Announcement{}).
			Where("source = ? AND status = ?", "seed", model.StatusPublished).
			Update("status", model.StatusOffline).Error
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
