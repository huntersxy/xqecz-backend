package services

import (
	"testing"
)

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		wantPlatform  VideoPlatform
		wantSourceID  string
		wantErrPrefix string
	}{
		{"empty URL", "", "", "", "URL为空"},
		{"invalid URL format", "://invalid", "", "", "无效的URL格式"},
		{"unsupported platform", "https://example.com/video/123", "", "", "不支持的视频平台"},
		{
			name:         "B站 BV号",
			url:          "https://www.bilibili.com/video/BV1GJ411x7h7",
			wantPlatform: PlatformBilibili,
			wantSourceID: "BV1GJ411x7h7",
		},
		{
			name:         "B站短链接",
			url:          "https://b23.tv/BV1xx411c7mD",
			wantPlatform: PlatformBilibili,
			wantSourceID: "BV1xx411c7mD",
		},
		{
			name:         "B站 URL参数",
			url:          "https://www.bilibili.com/video/BV1GJ411x7h7?p=1&t=30",
			wantPlatform: PlatformBilibili,
			wantSourceID: "BV1GJ411x7h7",
		},
		{
			name:         "抖音标准链接",
			url:          "https://www.douyin.com/video/1234567890123456789",
			wantPlatform: PlatformDouyin,
			wantSourceID: "1234567890123456789",
		},
	{
		name:         "抖音短链接(当前不支持)",
		url:          "https://v.douyin.com/abc123/",
		wantErrPrefix: "无法从链接中提取抖音视频ID",
	},
		{
			name:         "YouTube标准链接",
			url:          "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			wantPlatform: PlatformYouTube,
			wantSourceID: "dQw4w9WgXcQ",
		},
		{
			name:         "YouTube短链接",
			url:          "https://youtu.be/dQw4w9WgXcQ",
			wantPlatform: PlatformYouTube,
			wantSourceID: "dQw4w9WgXcQ",
		},
		{
			name:         "YouTube embed",
			url:          "https://www.youtube.com/embed/dQw4w9WgXcQ",
			wantPlatform: PlatformYouTube,
			wantSourceID: "dQw4w9WgXcQ",
		},
		{
			name:         "YouTube shorts",
			url:          "https://www.youtube.com/shorts/dQw4w9WgXcQ",
			wantPlatform: PlatformYouTube,
			wantSourceID: "dQw4w9WgXcQ",
		},
		{"B站无BV号", "https://www.bilibili.com/", "", "", "无法从链接中提取B站BV号"},
		{"抖音无视频ID", "https://www.douyin.com/discover", "", "", "无法从链接中提取抖音视频ID"},
		{"YouTube无视频ID", "https://www.youtube.com/feed/trending", "", "", "无法从链接中提取YouTube视频ID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			platform, sourceID, err := DetectPlatform(tt.url)
			if tt.wantErrPrefix != "" {
				if err == nil {
					t.Errorf("DetectPlatform(%q) expected error containing %q, got nil", tt.url, tt.wantErrPrefix)
					return
				}
				if !contains(err.Error(), tt.wantErrPrefix) {
					t.Errorf("DetectPlatform(%q) error = %q, want containing %q", tt.url, err.Error(), tt.wantErrPrefix)
				}
				return
			}
			if err != nil {
				t.Errorf("DetectPlatform(%q) unexpected error: %v", tt.url, err)
				return
			}
			if platform != tt.wantPlatform {
				t.Errorf("DetectPlatform(%q) platform = %q, want %q", tt.url, platform, tt.wantPlatform)
			}
			if sourceID != tt.wantSourceID {
				t.Errorf("DetectPlatform(%q) sourceID = %q, want %q", tt.url, sourceID, tt.wantSourceID)
			}
		})
	}
}

func TestExtractBVID(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"standard BV", "https://www.bilibili.com/video/BV1GJ411x7h7", "BV1GJ411x7h7"},
		{"BV with params", "https://www.bilibili.com/video/BV1xx411c7mD?p=2", "BV1xx411c7mD"},
		{"short link", "https://b23.tv/BV1GJ411x7h7", "BV1GJ411x7h7"},
		{"no BV", "https://www.bilibili.com/", ""},
		{"no BV in path", "https://www.bilibili.com/video/av170001", ""},
		{"lowercase bv", "https://www.bilibili.com/video/bv1gj411x7h7", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBVID(tt.url)
			if got != tt.want {
				t.Errorf("extractBVID(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestExtractDouyinID(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"standard", "https://www.douyin.com/video/1234567890123456789", "1234567890123456789"},
		{"with params", "https://www.douyin.com/video/1234567890123456789?modal_id=999", "1234567890123456789"},
		{"with path", "https://www.douyin.com/video/1234567890123456789/", "1234567890123456789"},
		{"modal_id only", "https://www.douyin.com/user/self?modal_id=1234567890123456789", "1234567890123456789"},
		{"no ID", "https://www.douyin.com/discover", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDouyinID(tt.url)
			if got != tt.want {
				t.Errorf("extractDouyinID(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestExtractYouTubeID(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"standard watch", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"short link", "https://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"embed", "https://www.youtube.com/embed/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"shorts", "https://www.youtube.com/shorts/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"with extra params", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=30", "dQw4w9WgXcQ"},
		{"with timestamp", "https://youtu.be/dQw4w9WgXcQ?t=30", "dQw4w9WgXcQ"},
		{"no ID", "https://www.youtube.com/feed/trending", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractYouTubeID(tt.url)
			if got != tt.want {
				t.Errorf("extractYouTubeID(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestVideoPlatformConstants(t *testing.T) {
	if PlatformBilibili != "bilibili" {
		t.Errorf("PlatformBilibili = %q, want %q", PlatformBilibili, "bilibili")
	}
	if PlatformDouyin != "douyin" {
		t.Errorf("PlatformDouyin = %q, want %q", PlatformDouyin, "douyin")
	}
	if PlatformYouTube != "youtube" {
		t.Errorf("PlatformYouTube = %q, want %q", PlatformYouTube, "youtube")
	}
}

func TestDefaultCoverFilename(t *testing.T) {
	if DefaultCoverFilename != "dy.webp" {
		t.Errorf("DefaultCoverFilename = %q, want %q", DefaultCoverFilename, "dy.webp")
	}
}

func TestExternalVideoConstants(t *testing.T) {
	if errCreateRequest != "创建请求失败: %w" {
		t.Errorf("errCreateRequest = %q, want %q", errCreateRequest, "创建请求失败: %w")
	}
	if headerUserAgent != "User-Agent" {
		t.Errorf("headerUserAgent = %q, want %q", headerUserAgent, "User-Agent")
	}
	if userAgentValue != "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36" {
		t.Errorf("userAgentValue mismatch")
	}
}

func TestVideoInfoStruct(t *testing.T) {
	info := VideoInfo{
		Platform: PlatformBilibili,
		Title:    "测试视频",
		CoverURL: "https://example.com/cover.jpg",
		SourceID: "BV1234567890",
	}
	if info.Platform != PlatformBilibili {
		t.Errorf("Platform = %q", info.Platform)
	}
	if info.Title != "测试视频" {
		t.Errorf("Title = %q", info.Title)
	}
	if info.CoverURL != "https://example.com/cover.jpg" {
		t.Errorf("CoverURL = %q", info.CoverURL)
	}
	if info.SourceID != "BV1234567890" {
		t.Errorf("SourceID = %q", info.SourceID)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
