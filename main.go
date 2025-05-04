package main

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/charmbracelet/log"
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
	Client          = req.C().ImpersonateChrome()
	WikiURL         = "https://search.mcmod.cn/s"
	TargetPath      = []string{ServerMust, ClientMust, ServerInvalid, ClientInvalid, ServerOptional, ClientOptional, Unknown}
)

func init() {
	for _, path := range TargetPath {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			os.Mkdir(path, os.ModePerm)
		}
	}
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
	log.Debugf("Mod name: %s", s)
	return s
}

func CopyFile(src, dst string) {
	srcFile, err := os.Open(src)
	if err != nil {
		log.Error(err)
		return
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dst)
	if err != nil {
		log.Error(err)
		return
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		log.Error(err)
		return
	}
	log.Debugf("Copied %s to %s", src, dst)
}

func main() {
	matches, err := filepath.Glob("mods/*.jar")
	if err != nil {
		log.Error(err)
		return
	}
	log.Infof("Found %d mods\n", len(matches))
	bar := progressbar.Default(int64((len(matches))))

	Client.ImpersonateChrome()
	Client.SetMaxConnsPerHost(1)

	for _, match := range matches {
		bar.Add(1)
		time.Sleep(1 * time.Second)
		modName := GetModName(match)
		r, err := Client.R().SetQueryParam("key", modName).Get(WikiURL)
		if err != nil {
			log.Error(err)
			CopyFile(match, filepath.Join(Unknown, filepath.Base(match)))
			continue
		}
		doc, err := goquery.NewDocumentFromReader(r.Body)
		if err != nil {
			log.Error(err)
			CopyFile(match, filepath.Join(Unknown, filepath.Base(match)))
			continue
		}
		// 获取模组页面链接
		val, exists := doc.Find(".head").First().Find("a").Eq(1).Attr("href")
		if !exists {
			log.Warnf("Mod %s not found", modName)
			CopyFile(match, filepath.Join(Unknown, filepath.Base(match)))
			continue
		}
		// 获取模组页面
		log.Debugf("Mod %s found, url: %s", modName, val)
		r, err = Client.R().Get(val)
		if err != nil {
			log.Error(err)
			CopyFile(match, filepath.Join(Unknown, filepath.Base(match)))
			continue
		}
		doc, err = goquery.NewDocumentFromReader(r.Body)
		if err != nil {
			log.Error(err)
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
			log.Warnf("Mod %s not matched", modName)
			CopyFile(match, filepath.Join(Unknown, filepath.Base(match)))
		}
	}
	log.Info("Done")

}
