package changelog

import (
	"fmt"
	"github.com/coreos/go-semver/semver"
	"github.com/idena-network/idena-indexer-api/app/service"
	"github.com/idena-network/idena-indexer-api/log"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ChangeLog struct {
	srcUrl              string
	changeLogsByVersion map[string]*service.ChangeLogData
	prevLen             int64
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

func (changeLog *ChangeLog) refresh() {
	resp, err := http.Get(changeLog.srcUrl)
	if err != nil {
		changeLog.logger.Error("Failed to get CHANGELOG file", "err", err)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		changeLog.logger.Error("Unable to read CHANGELOG file", "err", err)
		return
	}
	if resp.ContentLength == changeLog.prevLen {
		return
	}
	changeLog.prevLen = resp.ContentLength

	changeLogsByVersion, err := parseChangeLog(body)
	if err != nil {
		changeLog.logger.Error("Unable to parse CHANGELOG file", "err", err)
		return
	}

	changeLog.changeLogsByVersion = changeLogsByVersion
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
				parts := strings.Split(strings.TrimPrefix(line, "- "), " (")
				if len(parts) == 0 {
					continue
				}
				change := parts[0]
				for partIndex := 1; partIndex < len(parts)-1; partIndex++ {
					change += " (" + parts[partIndex]
				}
				res[v.String()].Changes = append(res[v.String()].Changes, change)
			}
			break
		}
	}
	return res, nil
}
