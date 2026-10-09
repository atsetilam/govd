package core

import (
	"testing"

	"github.com/govdbot/govd/internal/database"
	"github.com/govdbot/govd/internal/models"
)

func TestDownloadFormatRejectsEmptyURL(t *testing.T) {
	ctx := &models.ExtractorContext{
		FilesTracker: models.NewFilesTracker(),
	}
	format := &models.MediaFormat{
		FormatID: "video",
		Type:     database.MediaTypeVideo,
		URL:      []string{""},
	}

	_, err := downloadFormat(ctx, 0, format)
	if err == nil {
		t.Fatal("expected error for empty URL, got nil")
	}
}