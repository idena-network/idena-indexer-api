package changelog

import (
	"fmt"
	"github.com/coreos/go-semver/semver"
	"github.com/idena-network/idena-indexer-api/app/service"
	"github.com/idena-network/idena-indexer-api/log"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
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
	{ // todo
		changeLogsByVersion := make(map[string]*service.ChangeLogData)
		changeLogsByVersion["0.26.0"] = &service.ChangeLogData{
			Upgrade: 5,
			Changes: []string{
				"Implement delayed offline penalties",
				"Add StoreToIpfsTx",
				"Burn 5/6 of invitee stake in KillInviteeTx",
				"Encourage early invitations",
				"Check that 5 blocks without ceremonial txs go in a row to finish the validation ceremony ",
				"Fix events for pool rewards in oracle voting",
			},
		}
		changeLog.changeLogsByVersion = changeLogsByVersion
		return
	}

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

	// todo
	_ = string(body)
	changeLogsByVersion := make(map[string]*service.ChangeLogData)

	changeLog.changeLogsByVersion = changeLogsByVersion
}
