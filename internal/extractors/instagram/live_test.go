package instagram

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/govdbot/govd/internal/logger"
	"github.com/govdbot/govd/internal/models"
	"go.uber.org/zap"
	"github.com/govdbot/govd/internal/networking"
)

func liveCtx(id string, imp bool) *models.ExtractorContext {
	c, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	_ = cancel
	return &models.ExtractorContext{
		ContentID: id, ContentURL: "https://www.instagram.com/p/" + id,
		Extractor: Extractor, Context: c, FilesTracker: models.NewFilesTracker(),
		HTTPClient: networking.NewHTTPClient(&networking.NewHTTPClientOptions{Impersonate: imp}),
	}
}

func TestLive(t *testing.T) {
	logger.L = zap.NewNop().Sugar()
	id := os.Getenv("IGID")
	if id == "" {
		t.Skip()
	}
	for _, imp := range []bool{false, true} {
		for name, f := range map[string]func(*models.ExtractorContext) (*models.Media, error){"route": GetRouteMedia, "gql": GetGQLMedia, "embed": GetEmbedMedia, "igram": GetIGramPost} {
			m, err := f(liveCtx(id, imp))
			if err != nil {
				t.Logf("imp=%v %s: ERR %v", imp, name, err)
				continue
			}
			for _, it := range m.Items {
				for _, fm := range it.Formats {
					t.Logf("imp=%v %s: OK %s %s %.80s", imp, name, fm.FormatID, fm.Type, fm.URL[0])
				}
			}
			t.Logf("imp=%v %s: items=%d", imp, name, len(m.Items))
		}
	}
}
