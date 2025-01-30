package main

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gookit/slog"
	"github.com/gookit/slog/handler"
	"github.com/imroc/req/v3"
	"github.com/schollz/progressbar/v3"
)

const (
	ServerMust     = "服务端需装"
	ClientMust     = "客户端需装"
	ServerInvalid  = "服务端无效"
	ClientInvalid  = "客户端无效"
	ServerOptional = "服务端可选"
	ClientOptional = "客户端可选"
	Unknown        = "未识别"
)

var (
	BracketsPattern = regexp.MustCompile(`\[([^]]+)\]`)
	VersionPattern  = regexp.MustCompile(`\b\d+(\.\d+)+\b|\bv\d+(\.\d+)+\b`)
	Client          = req.C()
	WikiURL         = "https://search.mcmod.cn/s"
	Log             *slog.Logger
	TargetPath      = []string{ServerMust, ClientMust, ServerInvalid, ClientInvalid, ServerOptional, ClientOptional, Unknown}
)

func init() {
	for _, path := range TargetPath {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			os.Mkdir(path, os.ModePerm)
		}
	}

	slog.DefaultChannelName = "mcmod-classifier"
	newLogger := slog.New()
	defer newLogger.Flush()
	fileH, err := handler.NewFileHandler("mcmod-classifier.log")
	if err != nil {
		panic(err)
	}
	newLogger.AddHandler(fileH)
	Log = newLogger
}

func GetModName(path string) string {
	s := filepath.Base(path)
	s = strings.TrimSuffix(s, ".jar")
	s = strings.ToLower(s)

	if BracketsPattern.FindStringSubmatch(s) != nil {
		s = BracketsPattern.FindStringSubmatch(s)[1]
	}

	if strings.Contains(s, "-forge") {
		s = strings.Split(s, "-forge")[0]
	}
	if strings.Contains(s, "-fabric") {
		s = strings.Split(s, "-fabric")[0]
	}
	if strings.Contains(s, "-quilt") {
		s = strings.Split(s, "-quilt")[0]
	}
	if strings.Contains(s, "-neoforge") {
		s = strings.Split(s, "-neoforge")[0]
	}

	if strings.Contains(s, "-") {
		parts := VersionPattern.Split(s, -1)
		s = strings.TrimSuffix(parts[0], "-")
	}
	Log.Debugf("Mod name: %s", s)
	return s
}

func CopyFile(src, dst string) {
	srcFile, err := os.Open(src)
	if err != nil {
		Log.Error(err)
		return
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dst)
	if err != nil {
		Log.Error(err)
		return
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		Log.Error(err)
		return
	}
	Log.Debugf("Copied %s to %s", src, dst)
}

func main() {
	matches, err := filepath.Glob("mods/*.jar")
	if err != nil {
		slog.Error(err)
		return
	}
	slog.Infof("Found %d mods\n", len(matches))
	bar := progressbar.Default(int64((len(matches))))

	Client.ImpersonateChrome()
	Client.SetMaxConnsPerHost(1)

	for _, match := range matches {
		bar.Add(1)
		time.Sleep(1 * time.Second)
		modName := GetModName(match)
		r, err := Client.R().SetQueryParam("key", modName).Get(WikiURL)
		if err != nil {
			slog.Error(err)
			CopyFile(match, filepath.Join(Unknown, filepath.Base(match)))
			continue
		}
		doc, err := goquery.NewDocumentFromReader(r.Body)
		if err != nil {
			slog.Error(err)
			CopyFile(match, filepath.Join(Unknown, filepath.Base(match)))
			continue
		}
		// 获取模组页面链接
		val, exists := doc.Find(".head").First().Find("a").Eq(1).Attr("href")
		if !exists {
			Log.Warnf("Mod %s not found", modName)
			CopyFile(match, filepath.Join(Unknown, filepath.Base(match)))
			continue
		}
		// 获取模组页面
		Log.Debugf("Mod %s found, url: %s", modName, val)
		r, err = Client.R().Get(val)
		if err != nil {
			slog.Error(err)
			CopyFile(match, filepath.Join(Unknown, filepath.Base(match)))
			continue
		}
		doc, err = goquery.NewDocumentFromReader(r.Body)
		if err != nil {
			slog.Error(err)
			CopyFile(match, filepath.Join(Unknown, filepath.Base(match)))
			continue
		}
		// 获取模组运行环境
		modRunEnv := doc.Find(".class-info-left").Find(".col-lg-12").Find(".col-lg-4").Eq(2).Text()
		matched := false
		for _, path := range TargetPath {
			if strings.Contains(modRunEnv, path) {
				CopyFile(match, filepath.Join(path, filepath.Base(match)))
				matched = true
			}
		}
		if !matched {
			Log.Warnf("Mod %s not matched", modName)
			CopyFile(match, filepath.Join(Unknown, filepath.Base(match)))
		}
	}
	slog.Info("Done")

}
