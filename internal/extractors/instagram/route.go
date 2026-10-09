package instagram

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/govdbot/govd/internal/database"
	"github.com/govdbot/govd/internal/logger"
	"github.com/govdbot/govd/internal/models"
	"github.com/govdbot/govd/internal/networking"
	"github.com/govdbot/govd/internal/util"
)

// Instagram closed the logged-out GraphQL endpoint (401 require_login)
// and the embed page no longer carries the media data. The logged-out
// web page itself still loads the post through /ajax/route-definition/,
// whose streamed reply contains the full media object, including
// progressive mp4 URLs. This calls the same endpoint.

const (
	routeEndpoint = "https://www.instagram.com/ajax/route-definition/"
	routePrefix   = "for (;;);"
	webUserAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

type routeMedia struct {
	Code           string `json:"code"`
	MediaType      int    `json:"media_type"`
	OriginalWidth  int32  `json:"original_width"`
	OriginalHeight int32  `json:"original_height"`
	Caption        *struct {
		Text string `json:"text"`
	} `json:"caption"`
	ImageVersions2 *struct {
		Candidates []struct {
			URL string `json:"url"`
		} `json:"candidates"`
	} `json:"image_versions2"`
	VideoVersions []struct {
		URL string `json:"url"`
	} `json:"video_versions"`
	CarouselMedia []*routeMedia `json:"carousel_media"`
}

type routeChunk struct {
	Result *struct {
		Result *struct {
			Data *struct {
				Media *struct {
					Post *routeMedia `json:"if_not_gated_logged_out"`
				} `json:"xig_polaris_media"`
			} `json:"data"`
		} `json:"result"`
	} `json:"result"`
}

func (m *routeMedia) imageURL() string {
	if m.ImageVersions2 == nil {
		return ""
	}
	// candidates are ordered from the largest
	for _, c := range m.ImageVersions2.Candidates {
		if strings.TrimSpace(c.URL) != "" {
			return c.URL
		}
	}
	return ""
}

func (m *routeMedia) videoURL() string {
	for _, v := range m.VideoVersions {
		if strings.TrimSpace(v.URL) != "" {
			return v.URL
		}
	}
	return ""
}

func GetRouteMedia(ctx *models.ExtractorContext) (*models.Media, error) {
	post, err := GetRoutePost(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get route data: %w", err)
	}
	return ParseRouteMedia(ctx, post)
}

func GetRoutePost(ctx *models.ExtractorContext) (*routeMedia, error) {
	lsd := util.RandomBase64(20)
	csrf := util.RandomBase64(32)

	form := url.Values{}
	form.Set("route_url", "/p/"+ctx.ContentID+"/")
	form.Set("__d", "www")
	form.Set("__user", "0")
	form.Set("__a", "1")
	form.Set("__comet_req", "7")
	form.Set("lsd", lsd)

	resp, err := ctx.Fetch(
		http.MethodPost,
		routeEndpoint,
		&networking.RequestParams{
			Body: strings.NewReader(form.Encode()),
			Headers: map[string]string{
				"User-Agent":     webUserAgent,
				"Content-Type":   "application/x-www-form-urlencoded",
				"X-FB-LSD":       lsd,
				"X-CSRFToken":    csrf,
				"X-IG-D":         "www",
				"Cookie":         "csrftoken=" + csrf,
				"Origin":         "https://www.instagram.com",
				"Referer":        "https://www.instagram.com/p/" + ctx.ContentID + "/",
				"Sec-Fetch-Site": "same-origin",
				"Sec-Fetch-Mode": "cors",
				"Sec-Fetch-Dest": "empty",
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	logger.WriteFile("igroute_response", resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid response code: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return ParseRoutePost(body)
}

func ParseRoutePost(body []byte) (*routeMedia, error) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, routePrefix) {
			continue
		}
		var chunk routeChunk
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, routePrefix)), &chunk); err != nil {
			continue
		}
		if chunk.Result == nil || chunk.Result.Result == nil ||
			chunk.Result.Result.Data == nil || chunk.Result.Result.Data.Media == nil {
			continue
		}
		if post := chunk.Result.Result.Data.Media.Post; post != nil {
			return post, nil
		}
	}
	return nil, fmt.Errorf("media not found in route response")
}

func ParseRouteMedia(ctx *models.ExtractorContext, post *routeMedia) (*models.Media, error) {
	media := ctx.NewMedia()
	if post.Caption != nil {
		media.SetCaption(post.Caption.Text)
	}

	nodes := []*routeMedia{post}
	if len(post.CarouselMedia) > 0 {
		nodes = post.CarouselMedia
	}
	for _, node := range nodes {
		if videoURL := node.videoURL(); videoURL != "" {
			item := media.NewItem()
			format := &models.MediaFormat{
				FormatID:   "video",
				Type:       database.MediaTypeVideo,
				VideoCodec: database.MediaCodecAvc,
				AudioCodec: database.MediaCodecAac,
				URL:        []string{videoURL},
				Width:      node.OriginalWidth,
				Height:     node.OriginalHeight,
			}
			if thumb := node.imageURL(); thumb != "" {
				format.ThumbnailURL = []string{thumb}
			}
			item.AddFormats(format)
			continue
		}
		if node.MediaType == 2 {
			// a video whose file URL is not exposed to logged-out visitors
			return nil, util.ErrUnavailable
		}
		imageURL := node.imageURL()
		if imageURL == "" {
			continue
		}
		item := media.NewItem()
		item.AddFormats(&models.MediaFormat{
			FormatID: "image",
			Type:     database.MediaTypePhoto,
			URL:      []string{imageURL},
		})
	}
	if len(media.Items) == 0 {
		return nil, fmt.Errorf("no media items found")
	}
	return media, nil
}
