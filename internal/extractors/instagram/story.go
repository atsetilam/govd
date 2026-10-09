package instagram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/govdbot/govd/internal/database"
	"github.com/govdbot/govd/internal/models"
	"github.com/govdbot/govd/internal/networking"
	"github.com/govdbot/govd/internal/util"
)

// Stories are only served to logged-in sessions. They are read from the
// same private API the web app uses, with the session cookies from
// private/cookies/instagram.txt (sessionid and csrftoken are required).
//
// The cookies are used for this one request only: every other Instagram
// extractor, and the file download that follows, run without them.

const (
	igWebAppID     = "936619743392459"
	storyInfoURL   = "https://www.instagram.com/api/v1/media/%s/info/"
	storyReferrer  = "https://www.instagram.com/stories/%s/%s/"
	cookieSession  = "sessionid"
	cookieCSRF     = "csrftoken"
	storyUserAgent = webUserAgent
)

type storyInfoResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Items   []struct {
		MediaType      int   `json:"media_type"`
		OriginalWidth  int32 `json:"original_width"`
		OriginalHeight int32 `json:"original_height"`
		VideoVersions  []struct {
			URL string `json:"url"`
		} `json:"video_versions"`
		ImageVersions2 *struct {
			Candidates []struct {
				URL string `json:"url"`
			} `json:"candidates"`
		} `json:"image_versions2"`
	} `json:"items"`
}

// withoutCookies makes the context's HTTP client stop sending cookies.
func withoutCookies(ctx *models.ExtractorContext) {
	if ctx.HTTPClient == nil || len(ctx.HTTPClient.Cookies) == 0 {
		return
	}
	c := *ctx.HTTPClient
	c.Cookies = nil
	ctx.HTTPClient = &c
}

func GetStoryMedia(ctx *models.ExtractorContext) (*models.Media, error) {
	var csrf string
	hasSession := false
	if ctx.HTTPClient != nil {
		for _, c := range ctx.HTTPClient.Cookies {
			switch c.Name {
			case cookieSession:
				hasSession = c.Value != ""
			case cookieCSRF:
				csrf = c.Value
			}
		}
	}
	if !hasSession {
		return nil, util.ErrAuthenticationNeeded
	}
	username := ctx.MatchGroups["user"]
	headers := map[string]string{
		"User-Agent":       storyUserAgent,
		"X-IG-App-ID":      igWebAppID,
		"X-Requested-With": "XMLHttpRequest",
		"X-CSRFToken":      csrf,
		"Accept":           "*/*",
		"Referer":          fmt.Sprintf(storyReferrer, username, ctx.ContentID),
	}
	resp, err := ctx.Fetch(
		http.MethodGet,
		fmt.Sprintf(storyInfoURL, ctx.ContentID),
		&networking.RequestParams{Headers: headers},
	)
	// the session must not travel with anything that comes next
	withoutCookies(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		// session expired, logged out or challenged: do not retry
		return nil, util.ErrAuthenticationNeeded
	case http.StatusNotFound:
		return nil, util.ErrUnavailable
	default:
		return nil, fmt.Errorf("invalid response code: %s", resp.Status)
	}
	var info storyInfoResponse
	if err := json.Unmarshal(body, &info); err != nil {
		// a login or challenge page instead of JSON
		if strings.Contains(string(body[:min(len(body), 2048)]), "<html") {
			return nil, util.ErrAuthenticationNeeded
		}
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if info.Status != "ok" {
		if strings.Contains(info.Message, "login_required") || strings.Contains(info.Message, "checkpoint") {
			return nil, util.ErrAuthenticationNeeded
		}
		return nil, fmt.Errorf("instagram answered: %s", info.Message)
	}
	if len(info.Items) == 0 {
		return nil, util.ErrUnavailable
	}
	it := info.Items[0]

	media := ctx.NewMedia()
	item := media.NewItem()
	var imageURL string
	if it.ImageVersions2 != nil {
		for _, c := range it.ImageVersions2.Candidates {
			if strings.TrimSpace(c.URL) != "" {
				imageURL = c.URL
				break
			}
		}
	}
	if len(it.VideoVersions) > 0 && strings.TrimSpace(it.VideoVersions[0].URL) != "" {
		format := &models.MediaFormat{
			FormatID:   "video",
			Type:       database.MediaTypeVideo,
			VideoCodec: database.MediaCodecAvc,
			AudioCodec: database.MediaCodecAac,
			URL:        []string{it.VideoVersions[0].URL},
			Width:      it.OriginalWidth,
			Height:     it.OriginalHeight,
		}
		if imageURL != "" {
			format.ThumbnailURL = []string{imageURL}
		}
		item.AddFormats(format)
		return media, nil
	}
	if imageURL == "" {
		return nil, errors.New("no media found in story")
	}
	item.AddFormats(&models.MediaFormat{
		FormatID: "photo",
		Type:     database.MediaTypePhoto,
		URL:      []string{imageURL},
	})
	return media, nil
}
