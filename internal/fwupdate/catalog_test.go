package fwupdate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateURL(t *testing.T) {
	cases := []struct {
		source, url string
		ok          bool
	}{
		{SourceOfficial, "http://api-web.cdn.flydigi.com/devicefirmwares/2026/04/01/a.hex", true},
		{SourceOfficial, "https://flydigi.com/a.hex", true},
		{SourceOfficial, "http://evilflydigi.com/a.hex", false},
		{SourceOfficial, "http://flydigi.com.evil.com/a.hex", false},
		{SourceOfficial, "ftp://api.flydigi.com/a.hex", false},
		{SourceOfficial, "file:///C:/a.hex", false},
		{SourceCustom, CustomFirmwareBaseURL + "/a.hex", true},
		{SourceCustom, "http://raw.githubusercontent.com/TIANLI0/THRM/main/firmware/a.hex", false},
		{SourceCustom, "https://raw.githubusercontent.com/someone/else/main/firmware/a.hex", false},
		{SourceCustom, CustomFirmwareBaseURL + "/../../../../evil/repo/main/a.hex", false},
		{SourceCustom, CustomFirmwareBaseURL + "evil/a.hex", false},
		{SourceLocal, CustomFirmwareBaseURL + "/a.hex", false},
		{"", "https://flydigi.com/a.hex", false},
	}
	for _, c := range cases {
		if err := ValidateURL(c.source, c.url); (err == nil) != c.ok {
			t.Errorf("ValidateURL(%q, %q) err = %v, want ok=%v", c.source, c.url, err, c.ok)
		}
	}
}

func TestVerifySHA256(t *testing.T) {
	data := []byte("abc")
	const sum = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if err := VerifySHA256(data, sum); err != nil {
		t.Errorf("正确的哈希应通过: %v", err)
	}
	if err := VerifySHA256(data, " "+strings.ToUpper(sum)+"\n"); err != nil {
		t.Errorf("大小写与空白不应影响比对: %v", err)
	}
	for _, bad := range []string{"", "abc", strings.Repeat("0", 64), strings.Repeat("z", 64)} {
		if err := VerifySHA256(data, bad); err == nil {
			t.Errorf("VerifySHA256(%q) 应失败", bad)
		}
	}
}

// 清单里地址不在仓库 firmware 目录下、或没有 sha256 的条目必须被丢弃。
func TestFetchCustomDropsUnverifiableEntries(t *testing.T) {
	good := FirmwareInfo{Version: "1", URL: CustomFirmwareBaseURL + "/ok.hex", SHA256: strings.Repeat("AB", 32)}
	list := []FirmwareInfo{
		good,
		{Version: "2", URL: CustomFirmwareBaseURL + "/nohash.hex"},
		{Version: "3", URL: "https://example.com/x.hex", SHA256: strings.Repeat("ab", 32)},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(list)
	}))
	defer srv.Close()

	got, err := FetchCustom(context.Background(), srv.URL, "bs3pro")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Version != "1" {
		t.Fatalf("got %+v, want only version 1", got)
	}
	if got[0].Source != SourceCustom || got[0].SHA256 != strings.Repeat("ab", 32) {
		t.Errorf("条目未归一化: %+v", got[0])
	}
}

func TestDownloadRejectsOversized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, maxFirmwareDownload+1))
	}))
	defer srv.Close()
	if _, err := Download(context.Background(), srv.URL); err == nil {
		t.Fatal("超过上限的下载应失败")
	}
}

// 仓库 firmware/ 下的清单是线上客户端实际读取的内容：每个条目都必须能通过
// 客户端的校验，且 sha256 与仓库里的固件文件一致。
func TestRepositoryManifests(t *testing.T) {
	dir := filepath.Join("..", "..", "firmware")
	manifests, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(manifests) == 0 {
		t.Fatalf("没有找到固件清单: %v", err)
	}
	for _, m := range manifests {
		raw, err := os.ReadFile(m)
		if err != nil {
			t.Fatal(err)
		}
		var list []FirmwareInfo
		if err := json.Unmarshal(raw, &list); err != nil {
			t.Fatalf("%s: %v", m, err)
		}
		for _, fw := range list {
			if err := ValidateURL(SourceCustom, fw.URL); err != nil {
				t.Errorf("%s %s: %v", m, fw.Version, err)
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, path.Base(fw.URL)))
			if err != nil {
				t.Errorf("%s %s: 清单指向的固件文件不在仓库里: %v", m, fw.Version, err)
				continue
			}
			// .gitattributes 没有给 .hex 指定换行，Windows 检出可能是 CRLF；线上分发的是 LF。
			data = []byte(strings.ReplaceAll(string(data), "\r\n", "\n"))
			if err := VerifySHA256(data, fw.SHA256); err != nil {
				t.Errorf("%s %s: %v", m, fw.Version, err)
			}
		}
	}
}
