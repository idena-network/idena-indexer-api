package changelog

import (
	"fmt"
	"github.com/coreos/go-semver/semver"
	"github.com/idena-network/idena-indexer-api/app/service"
	"github.com/idena-network/idena-indexer-api/log"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ChangeLog struct {
	srcUrl              string
	changeLogsByVersion map[string]*service.ChangeLogData
	urlsByUpgrade       map[uint32]string
	prevLen             int
	logger              log.Logger
}

func NewChangeLog(srcUrl string, logger log.Logger) *ChangeLog {
	res := &ChangeLog{
		srcUrl:              srcUrl,
		changeLogsByVersion: make(map[string]*service.ChangeLogData),
		urlsByUpgrade:       make(map[uint32]string),
		logger:              logger,
	}
	go res.loopRefreshing()
	return res
}

func (changeLog *ChangeLog) ForkChangeLog(version string) (*service.ChangeLogData, error) {
	v, err := semver.NewVersion(version)
	if err != nil {
		return nil, errors.New("invalid version")
	}
	forkVersion := fmt.Sprintf("%v.%v.0", v.Major, v.Minor)
	return changeLog.changeLogsByVersion[forkVersion], nil
}

func (changeLog *ChangeLog) Url(upgrade uint32) string {
	return changeLog.urlsByUpgrade[upgrade]
}

func (changeLog *ChangeLog) loopRefreshing() {
	for {
		changeLog.refresh()
		time.Sleep(time.Minute * 5)
	}
}

func getData(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (changeLog *ChangeLog) refresh() {
	mainData, err := getData(changeLog.srcUrl)
	if err != nil {
		changeLog.logger.Error("Unable to get CHANGELOG main file", "err", err)
		return
	}
	if len(mainData) == changeLog.prevLen {
		return
	}
	changeLog.prevLen = len(mainData)
	links, linksByVersion, err := parseMainChangeLog(mainData)
	if err != nil {
		changeLog.logger.Error("Unable to parse CHANGELOG main file", "err", err)
		return
	}

	var all []byte
	for i := len(links) - 1; i >= 0; i-- {
		url := generateUrl(changeLog.srcUrl, links[i], "")
		data, err := getData(url)
		if err != nil {
			changeLog.logger.Error("Unable to get CHANGELOG file", "err", err, "url", url)
			return
		}
		all = append(all, data...)
	}

	changeLogsByVersion, urlsByUpgrade, err := parseChangeLog(all, linksByVersion)
	if err != nil {
		changeLog.logger.Error("Unable to parse changelogs", "err", err)
		return
	}

	changeLog.changeLogsByVersion = changeLogsByVersion
	changeLog.urlsByUpgrade = urlsByUpgrade
}

func parseMainChangeLog(data []byte) ([]string, map[string]string, error) {
	strData := string(data)
	lines := strings.Split(strData, "\n")
	var links []string
	linksByVersion := make(map[string]string)
	for _, line := range lines {
		if !strings.HasPrefix(line, "-") {
			continue
		}
		v := regexp.MustCompile(`\(.*\)`).FindString(line)
		url := v[1 : len(v)-1]
		if len(v) > 2 {
			links = append(links, url)
			v = regexp.MustCompile(`\[.*]`).FindString(line)
			if len(v) > 2 {
				ver := v[1 : len(v)-1]
				linksByVersion[ver] = url
			}
		}
	}
	return links, linksByVersion, nil
}

func parseChangeLog(data []byte, linksByVersion map[string]string) (map[string]*service.ChangeLogData, map[uint32]string, error) {
	strData := string(data)
	lines := strings.Split(strData, "\n")
	res := make(map[string]*service.ChangeLogData)
	urlsByUpgrade := make(map[uint32]string)
	var i int
	for i < len(lines) {
		line := lines[i]
		i++
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		s := strings.Split(strings.TrimPrefix(line, "## "), " ")
		if len(s) == 0 {
			continue
		}
		v, err := semver.NewVersion(s[0])
		if err != nil || v.Patch != 0 {
			continue
		}
		anchor, err := generateUrlAnchor(v, line)
		if err != nil {
			continue
		}
		for i < len(lines) {
			line = lines[i]
			i++
			if !strings.HasPrefix(line, "### Fork (Upgrade ") {
				continue
			}
			upgrade, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(line, "### Fork (Upgrade "), ")"))
			if err != nil {
				continue
			}
			res[v.String()] = &service.ChangeLogData{
				Upgrade: uint32(upgrade),
			}
			major := v.String()[0:strings.LastIndex(v.String(), ".")]
			if link, ok := linksByVersion[fmt.Sprintf("v%v", major)]; ok {
				url := generateUrl("https://github.com/idena-network/idena-go/blob/master/CHANGELOG.md", link, anchor)
				res[v.String()].Url = url
				urlsByUpgrade[uint32(upgrade)] = url
			}
			for i < len(lines) {
				line = lines[i]
				i++
				if strings.HasPrefix(line, "#") {
					break
				}
				if !strings.HasPrefix(line, "- ") {
					continue
				}
				line = strings.TrimPrefix(line, "- ")
				r := regexp.MustCompile(` \(\[#+[0-9]*]\)`)
				change := r.ReplaceAllString(line, "")
				res[v.String()].Changes = append(res[v.String()].Changes, change)
			}
			break
		}
	}
	return res, urlsByUpgrade, nil
}

func generateUrl(baseLink, localLink, anchor string) string {
	return strings.Replace(baseLink, "CHANGELOG.md", "", 1) + localLink + "#" + anchor
}

func generateUrlAnchor(ver *semver.Version, v string) (string, error) {
	sub := regexp.MustCompile(`\(.*\)`).FindString(v)
	if len(sub) <= 2 {
		return "", errors.Errorf("failed to extract anchor from: %v", v)
	}
	date := sub[1 : len(sub)-1]
	return fmt.Sprintf("%v%v%v-%v", ver.Major, ver.Minor, ver.Patch, strings.Replace(strings.Replace(date, ", ", "-", 1), " ", "-", 1)), nil
}
