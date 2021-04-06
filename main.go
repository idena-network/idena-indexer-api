package main

import (
	api "github.com/idena-network/idena-indexer-api/app"
	"github.com/idena-network/idena-indexer-api/config"
	"github.com/idena-network/idena-indexer-api/log"
	"gopkg.in/urfave/cli.v1"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// @license.name Apache 2.0
func main() {
	app := cli.NewApp()
	app.Name = "github.com/idena-network/idena-indexer-api"
	app.Version = "0.1.0"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config",
			Usage: "Config file",
			Value: filepath.Join("conf", "config.json"),
		},
	}

	app.Action = func(context *cli.Context) error {
		conf := config.LoadConfig(context.String("config"))
		initLog(conf.Verbosity)
		log.Info("Initializing app...")
		apiApp := api.InitializeApp(conf, conf.MaxReqCount, time.Second*time.Duration(conf.ReqTimeoutSec), conf.ReqsPerMinuteLimit)
		defer apiApp.Destroy()
		log.Info("Starting server...")
		apiApp.Start(conf.Swagger)
		return nil
	}

	app.Run(os.Args)
}

func initLog(verbosity int) {
	logLvl := log.Lvl(verbosity)
	var handler log.Handler
	if runtime.GOOS == "windows" {
		handler = log.LvlFilterHandler(logLvl, log.StreamHandler(os.Stdout, log.LogfmtFormat()))
	} else {
		handler = log.LvlFilterHandler(logLvl, log.StreamHandler(os.Stdout, log.TerminalFormat(true)))
	}
	log.Root().SetHandler(handler)
}
