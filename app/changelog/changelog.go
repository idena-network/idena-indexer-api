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
	prevLen             int
	logger              log.Logger
}

func NewChangeLog(srcUrl string, logger log.Logger) *ChangeLog {
	res := &ChangeLog{
		srcUrl:              srcUrl,
		changeLogsByVersion: make(map[string]*service.ChangeLogData),
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
	urls, err := parseMainChangeLog(mainData)
	if err != nil {
		changeLog.logger.Error("Unable to parse CHANGELOG main file", "err", err)
		return
	}

	var all []byte
	for i := len(urls) - 1; i >= 0; i-- {
		url := strings.Replace(changeLog.srcUrl, "CHANGELOG.md", "", 1) + urls[i]
		data, err := getData(url)
		if err != nil {
			changeLog.logger.Error("Unable to get CHANGELOG file", "err", err, "url", url)
			return
		}
		all = append(all, data...)
	}

	changeLogsByVersion, err := parseChangeLog(all)
	if err != nil {
		changeLog.logger.Error("Unable to parse changelogs", "err", err)
		return
	}

	changeLog.changeLogsByVersion = changeLogsByVersion
}

func parseMainChangeLog(data []byte) ([]string, error) {
	strData := string(data)
	lines := strings.Split(strData, "\n")
	var res []string
	for _, line := range lines {
		if !strings.HasPrefix(line, "-") {
			continue
		}
		v := regexp.MustCompile(`\(.*\)`).FindString(line)
		if len(v) > 2 {
			res = append(res, v[1:len(v)-1])
		}
	}
	return res, nil
}

func parseChangeLog(data []byte) (map[string]*service.ChangeLogData, error) {
	strData := string(data)
	lines := strings.Split(strData, "\n")
	res := make(map[string]*service.ChangeLogData)
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
	return res, nil
}
