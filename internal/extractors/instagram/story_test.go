package instagram

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/govdbot/govd/internal/logger"
	"github.com/govdbot/govd/internal/models"
	"github.com/govdbot/govd/internal/networking"
	"github.com/govdbot/govd/internal/util"
	"go.uber.org/zap"
)

type recorder struct {
	cookies []string
	status  int
	body    string
}

func (r *recorder) Do(req *http.Request) (*http.Response, error) {
	r.cookies = append(r.cookies, req.Header.Get("Cookie"))
	return &http.Response{
		StatusCode: r.status,
		Status:     http.StatusText(r.status),
		Body:       io.NopCloser(strings.NewReader(r.body)),
		Header:     http.Header{},
		Request:    req,
	}, nil
}

func fakeCtx(rec *recorder, ext *models.Extractor, id string) *models.ExtractorContext {
	return &models.ExtractorContext{
		ContentID:    id,
		ContentURL:   "https://www.instagram.com/stories/someone/" + id,
		MatchGroups:  map[string]string{"user": "someone"},
		Extractor:    ext,
		Context:      context.Background(),
		FilesTracker: models.NewFilesTracker(),
		HTTPClient: &networking.HTTPClient{
			Client: rec,
			Cookies: []*http.Cookie{
				{Name: "sessionid", Value: "test-session"},
				{Name: "csrftoken", Value: "test-csrf"},
			},
		},
	}
}

func TestPostsNeverSendCookies(t *testing.T) {
	logger.L = zap.NewNop().Sugar()
	rec := &recorder{status: http.StatusInternalServerError}
	ctx := fakeCtx(rec, Extractor, "abc")
	_, _ = Extractor.GetFunc(ctx)
	if len(rec.cookies) == 0 {
		t.Fatal("no requests were made")
	}
	for _, c := range rec.cookies {
		if strings.Contains(c, "sessionid") {
			t.Fatalf("session cookie sent on a post request: %q", c)
		}
	}
}

func TestStoryUsesCookiesOnceThenDropsThem(t *testing.T) {
	logger.L = zap.NewNop().Sugar()
	rec := &recorder{status: http.StatusOK, body: `{"status":"ok","items":[{"media_type":1,"image_versions2":{"candidates":[{"url":"https://example.invalid/a.jpg"}]}}]}`}
	ctx := fakeCtx(rec, StoriesExtractor, "123")
	m, err := GetStoryMedia(ctx)
	if err != nil || len(m.Items) != 1 {
		t.Fatalf("unexpected result: %v", err)
	}
	if len(rec.cookies) != 1 || !strings.Contains(rec.cookies[0], "sessionid") {
		t.Fatalf("story request should carry the session: %v", rec.cookies)
	}
	if len(ctx.HTTPClient.Cookies) != 0 {
		t.Fatal("cookies must be dropped before the download")
	}
}

func TestStoryWithoutSessionOrExpired(t *testing.T) {
	logger.L = zap.NewNop().Sugar()
	rec := &recorder{status: http.StatusOK}
	ctx := fakeCtx(rec, StoriesExtractor, "123")
	ctx.HTTPClient.Cookies = nil
	if _, err := GetStoryMedia(ctx); err != util.ErrAuthenticationNeeded {
		t.Fatalf("want authentication needed, got %v", err)
	}
	if len(rec.cookies) != 0 {
		t.Fatal("no request should be made without a session")
	}
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		rec := &recorder{status: status, body: `{"message":"login_required"}`}
		if _, err := GetStoryMedia(fakeCtx(rec, StoriesExtractor, "123")); err != util.ErrAuthenticationNeeded {
			t.Fatalf("status %d: want authentication needed, got %v", status, err)
		}
	}
}

// live check against the real cookie file; set IGSTORY to a story URL
// (needs private/cookies/instagram.txt, run from the repository root).
func TestLiveStory(t *testing.T) {
	url := os.Getenv("IGSTORY")
	if url == "" {
		t.Skip()
	}
	logger.L = zap.NewNop().Sugar()
	if err := os.Chdir("../../.."); err != nil {
		t.Fatal(err)
	}
	m := StoriesExtractor.URLPattern.FindStringSubmatch(url)
	if m == nil {
		t.Fatal("not a story url")
	}
	ctx := &models.ExtractorContext{
		ContentID:    m[StoriesExtractor.URLPattern.SubexpIndex("id")],
		ContentURL:   url,
		MatchGroups:  map[string]string{"user": m[StoriesExtractor.URLPattern.SubexpIndex("user")]},
		Extractor:    StoriesExtractor,
		Context:      context.Background(),
		FilesTracker: models.NewFilesTracker(),
		HTTPClient:   networking.NewHTTPClient(&networking.NewHTTPClientOptions{Cookies: util.GetExtractorCookies("instagram")}),
	}
	media, err := GetStoryMedia(ctx)
	if err != nil {
		t.Fatalf("story failed: %v", err)
	}
	for _, it := range media.Items {
		for _, f := range it.Formats {
			t.Logf("OK %s %s %dx%d", f.FormatID, f.Type, f.Width, f.Height)
			// the file itself is fetched without any cookie
			r, err := http.Get(f.URL[0])
			if err != nil {
				t.Fatal(err)
			}
			n, _ := io.Copy(io.Discard, r.Body)
			r.Body.Close()
			t.Logf("download: %d %s %d bytes", r.StatusCode, r.Header.Get("Content-Type"), n)
		}
	}
}
