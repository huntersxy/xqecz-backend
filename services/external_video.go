package services

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	json "github.com/bytedance/sonic"
	"xiaoquan-backend/config"
)

const (
	DefaultCoverFilename = "dy.webp"

	errCreateRequest = "创建请求失败: %w"
	headerUserAgent  = "User-Agent"
	userAgentValue   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
)

type VideoPlatform string

const (
	PlatformBilibili VideoPlatform = "bilibili"
	PlatformDouyin   VideoPlatform = "douyin"
	PlatformYouTube  VideoPlatform = "youtube"
)

type VideoInfo struct {
	Platform VideoPlatform `json:"platform"`
	Title    string        `json:"title"`
	CoverURL string        `json:"cover_url"`
	SourceID string        `json:"source_id"`
}

var (
	bilibiliBVPattern  = regexp.MustCompile(`BV[0-9A-Za-z]{10}`)
	douyinVideoPattern = regexp.MustCompile(`(?:www\.)?douyin\.com/video/(\d+)`)
	douyinShortPattern = regexp.MustCompile(`v\.douyin\.com/([A-Za-z0-9]+)`)
	youtubeIDPattern   = regexp.MustCompile(`(?:youtube\.com/watch\?v=|youtu\.be/|youtube\.com/embed/|youtube\.com/v/|youtube\.com/shorts/)([A-Za-z0-9_-]{11})`)
)

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     60 * time.Second,
	},
}

func EnsureDefaultCover(data []byte) {
	dest := filepath.Join(config.AppConfig.Server.ThumbnailDir, DefaultCoverFilename)
	if _, err := os.Stat(dest); err == nil {
		return
	}
	if err := os.WriteFile(dest, data, 0644); err != nil {
		log.Printf("[封面] 写入默认封面失败: %v", err)
		return
	}
	log.Printf("[封面] 已部署默认封面: %s", dest)
}

func DetectPlatform(rawURL string) (VideoPlatform, string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", "", fmt.Errorf("URL为空")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("无效的URL格式")
	}

	host := strings.ToLower(parsed.Hostname())

	if strings.Contains(host, "bilibili.com") || strings.Contains(host, "b23.tv") {
		bvid := extractBVID(rawURL)
		if bvid == "" {
			return "", "", fmt.Errorf("无法从链接中提取B站BV号")
		}
		return PlatformBilibili, bvid, nil
	}

	if strings.Contains(host, "douyin.com") {
		vid := extractDouyinID(rawURL)
		if vid == "" {
			return "", "", fmt.Errorf("无法从链接中提取抖音视频ID")
		}
		return PlatformDouyin, vid, nil
	}

	if strings.Contains(host, "youtube.com") || strings.Contains(host, "youtu.be") {
		vid := extractYouTubeID(rawURL)
		if vid == "" {
			return "", "", fmt.Errorf("无法从链接中提取YouTube视频ID")
		}
		return PlatformYouTube, vid, nil
	}

	return "", "", fmt.Errorf("不支持的视频平台，当前支持：B站(bilibili)、抖音(douyin)、YouTube")
}

func ValidateExternalURL(rawURL string) (*VideoInfo, error) {
	platform, sourceID, err := DetectPlatform(rawURL)
	if err != nil {
		return nil, err
	}

	info, err := FetchVideoInfo(platform, sourceID)
	if err != nil {
		return nil, fmt.Errorf("获取视频信息失败: %w", err)
	}

	return info, nil
}

func FetchVideoInfo(platform VideoPlatform, sourceID string) (*VideoInfo, error) {
	switch platform {
	case PlatformBilibili:
		return fetchBilibiliInfo(sourceID)
	case PlatformDouyin:
		return fetchDouyinInfo(sourceID)
	case PlatformYouTube:
		return fetchYouTubeInfo(sourceID)
	default:
		return nil, fmt.Errorf("不支持的平台: %s", platform)
	}
}

func fetchBilibiliInfo(bvid string) (*VideoInfo, error) {
	apiURL := fmt.Sprintf("https://api.bilibili.com/x/web-interface/view?bvid=%s", bvid)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf(errCreateRequest, err)
	}
	req.Header.Set(headerUserAgent, userAgentValue)
	req.Header.Set("Referer", "https://www.bilibili.com")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求B站API失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取API响应失败: %w", err)
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Bvid  string `json:"bvid"`
			Title string `json:"title"`
			Pic   string `json:"pic"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析API响应失败: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("B站API错误: %s (code=%d)", result.Message, result.Code)
	}

	if result.Data.Bvid == "" {
		return nil, fmt.Errorf("B站API未返回视频数据")
	}

	return &VideoInfo{
		Platform: PlatformBilibili,
		Title:    result.Data.Title,
		CoverURL: result.Data.Pic,
		SourceID: result.Data.Bvid,
	}, nil
}

func fetchDouyinInfo(videoID string) (*VideoInfo, error) {
	apiURL := fmt.Sprintf("https://www.douyin.com/video/%s", videoID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf(errCreateRequest, err)
	}
	req.Header.Set(headerUserAgent, userAgentValue)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求抖音页面失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取抖音页面失败: %w", err)
	}

	html := string(body)

	titlePattern := regexp.MustCompile(`<title[^>]*>([^<]+)</title>`)
	coverPattern := regexp.MustCompile(`"coverUrl"\s*:\s*"([^"]+)"`)
	ogImagePattern := regexp.MustCompile(`<meta[^>]+property="og:image"[^>]+content="([^"]+)"`)
	ogTitlePattern := regexp.MustCompile(`<meta[^>]+property="og:title"[^>]+content="([^"]+)"`)

	title := videoID
	if m := ogTitlePattern.FindStringSubmatch(html); len(m) >= 2 {
		title = m[1]
	} else if m := titlePattern.FindStringSubmatch(html); len(m) >= 2 {
		title = strings.NewReplacer("抖音-", "", " - 抖音", "").Replace(strings.TrimSpace(m[1]))
	}

	coverURL := ""
	if m := ogImagePattern.FindStringSubmatch(html); len(m) >= 2 {
		coverURL = m[1]
	} else if m := coverPattern.FindStringSubmatch(html); len(m) >= 2 {
		coverURL = m[2]
	}

	if coverURL == "" {
		log.Printf("[抖音] 未提取到封面图，视频ID: %s", videoID)
	}

	return &VideoInfo{
		Platform: PlatformDouyin,
		Title:    title,
		CoverURL: coverURL,
		SourceID: videoID,
	}, nil
}

func fetchYouTubeInfo(videoID string) (*VideoInfo, error) {
	apiURL := fmt.Sprintf("https://www.youtube.com/oembed?url=https://www.youtube.com/watch?v=%s&format=json", videoID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf(errCreateRequest, err)
	}
	req.Header.Set(headerUserAgent, userAgentValue)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求YouTube API失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取API响应失败: %w", err)
	}

	var result struct {
		Title          string `json:"title"`
		ThumbnailURL   string `json:"thumbnail_url"`
		ThumbnailWidth int    `json:"thumbnail_width"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析API响应失败: %w", err)
	}

	if result.Title == "" {
		return nil, fmt.Errorf("YouTube未返回视频数据，请检查视频ID是否正确")
	}

	coverURL := result.ThumbnailURL
	if coverURL == "" {
		coverURL = fmt.Sprintf("https://img.youtube.com/vi/%s/maxresdefault.jpg", videoID)
	}

	return &VideoInfo{
		Platform: PlatformYouTube,
		Title:    result.Title,
		CoverURL: coverURL,
		SourceID: videoID,
	}, nil
}

func DownloadCover(coverURL string, userID uint, fallback bool) (string, error) {
	if coverURL == "" {
		if fallback {
			return DefaultCoverFilename, nil
		}
		return "", fmt.Errorf("封面URL为空")
	}

	req, err := http.NewRequest("GET", coverURL, nil)
	if err != nil {
		return DefaultCoverFilename, nil
	}
	req.Header.Set(headerUserAgent, userAgentValue)
	req.Header.Set("Referer", coverURL)

	resp, err := httpClient.Do(req)
	if err != nil {
		if fallback {
			return DefaultCoverFilename, nil
		}
		return "", fmt.Errorf("下载封面失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if fallback {
			return DefaultCoverFilename, nil
		}
		return "", fmt.Errorf("下载封面失败，HTTP状态码: %d", resp.StatusCode)
	}

	ts := time.Now().UnixNano()
	tmpFilename := fmt.Sprintf("%d_%d_thumb_tmp", userID, ts)
	tmpPath := filepath.Join(config.AppConfig.Server.UploadDir, tmpFilename)

	f, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("创建封面文件失败: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return "", fmt.Errorf("保存封面文件失败: %w", err)
	}
	f.Close()

	webpFilename := fmt.Sprintf("%d_%d_thumb.webp", userID, ts)
	webpPath := filepath.Join(config.AppConfig.Server.ThumbnailDir, webpFilename)

	if err := convertToWebP(tmpPath, webpPath); err != nil {
		log.Printf("[封面] 转WebP失败，保留原始格式: %v", err)
		os.Remove(webpPath)
		jpgFilename := fmt.Sprintf("%d_%d_thumb.jpg", userID, ts)
		jpgPath := filepath.Join(config.AppConfig.Server.ThumbnailDir, jpgFilename)
		os.Rename(tmpPath, jpgPath)
		return jpgFilename, nil
	}

	os.Remove(tmpPath)
	return webpFilename, nil
}

func convertToWebP(srcPath, destPath string) error {
	ffmpeg, _ := exec.LookPath("ffmpeg")
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	cmd := exec.Command(ffmpeg, "-i", srcPath, "-c:v", "libwebp", "-quality", "60", "-y", destPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg转webp失败: %w", err)
	}
	return nil
}

func extractBVID(rawURL string) string {
	if match := bilibiliBVPattern.FindString(rawURL); match != "" {
		return match
	}
	parsed, _ := url.Parse(rawURL)
	if parsed != nil {
		if match := bilibiliBVPattern.FindString(parsed.Path); match != "" {
			return match
		}
	}
	return ""
}

func extractDouyinID(rawURL string) string {
	if m := douyinVideoPattern.FindStringSubmatch(rawURL); len(m) >= 2 {
		return m[1]
	}
	parsed, _ := url.Parse(rawURL)
	if parsed != nil {
		if modalID := parsed.Query().Get("modal_id"); modalID != "" {
			return modalID
		}
	}
	return ""
}

func extractYouTubeID(rawURL string) string {
	if m := youtubeIDPattern.FindStringSubmatch(rawURL); len(m) >= 2 {
		return m[1]
	}
	parsed, _ := url.Parse(rawURL)
	if parsed != nil {
		queryID := parsed.Query().Get("v")
		if queryID != "" {
			return queryID
		}
	}
	return ""
}
