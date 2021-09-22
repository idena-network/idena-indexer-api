package app

import (
	"fmt"
	"github.com/idena-network/idena-indexer-api/app/api"
	"github.com/idena-network/idena-indexer-api/app/changelog"
	"github.com/idena-network/idena-indexer-api/app/db"
	"github.com/idena-network/idena-indexer-api/app/db/cached"
	"github.com/idena-network/idena-indexer-api/app/db/postgres"
	logUtil "github.com/idena-network/idena-indexer-api/app/log"
	"github.com/idena-network/idena-indexer-api/app/monitoring"
	service2 "github.com/idena-network/idena-indexer-api/app/service"
	"github.com/idena-network/idena-indexer-api/config"
	"github.com/idena-network/idena-indexer-api/indexer"
	"github.com/idena-network/idena-indexer-api/log"
	"time"
)

type App interface {
	Start(swaggerConfig config.SwaggerConfig)
	Destroy()
}

func InitializeApp(
	conf *config.Config,
	maxReqCount int,
	timeout time.Duration,
	reqsPerMinuteLimit int,
) App {
	indexerLogger, err := logUtil.NewFileLogger("indexer.log", conf.LogFileSize)
	if err != nil {
		panic(fmt.Sprintf("unable to initialize indexer logger: %v", err))
	}
	indexerClient := indexer.NewClient(conf.Indexer.Url, conf.Indexer.MaxConnections)
	indexerApi := indexer.NewApi(indexerClient, indexerLogger)
	cachedNetworkSizeLoader := service2.NewCachedNetworkSizeLoader(indexer.NewNetworkSizeLoader(indexerApi))

	memPool := indexer.NewMemPool(indexerApi)
	contractsMemPool := indexer.NewContractsMemPool(indexerApi)

	logger, err := logUtil.NewFileLogger("api.log", conf.LogFileSize)
	if err != nil {
		panic(err)
	}
	pm, err := createPerformanceMonitor(conf.PerformanceMonitor)
	if err != nil {
		panic(err)
	}
	accessor := cached.NewCachedAccessor(
		postgres.NewPostgresAccessor(
			conf.PostgresConnStr,
			conf.ScriptsDir,
			conf.DynamicEndpointsTable,
			conf.DynamicEndpointStatesTable,
			cachedNetworkSizeLoader,
			logger,
		),
		conf.DefaultCacheMaxItemCount,
		time.Second*time.Duration(conf.DefaultCacheItemLifeTimeSec),
		logger.New("component", "cachedDbAccessor"),
	)
	changeLog := changelog.NewChangeLog(conf.ChangeLogUrl, logger.New("component", "changeLog"))
	service := api.NewService(accessor, memPool, indexerApi, changeLog)
	contractsService := service2.NewContracts(accessor, contractsMemPool)
	dynamicConfigHolder := config.NewDynamicConfigHolder(conf.DynamicConfigFile, logger.New("component", "dConfHolder"))
	var dynamicEndpointLoader service2.DynamicEndpointLoader
	if len(conf.DynamicEndpointsTable) > 0 {
		dynamicEndpointLoader = service2.NewDynamicEndpointLoader(accessor)
	}
	app := &app{
		server: api.NewServer(
			conf.Port,
			conf.LatestHours,
			conf.ActiveAddressHours,
			conf.FrozenBalanceAddrs,
			func() string {
				c := dynamicConfigHolder.GetConfig()
				if c == nil || len(c.DumpCid) == 0 {
					return ""
				}
				return fmt.Sprintf("https://ipfs.io/ipfs/%s", c.DumpCid)
			},
			service,
			contractsService,
			logger,
			pm,
			maxReqCount,
			timeout,
			reqsPerMinuteLimit,
			dynamicEndpointLoader,
			conf.Cors,
		),
		db:     accessor,
		logger: logger,
	}
	return app
}

func createPerformanceMonitor(c config.PerformanceMonitorConfig) (monitoring.PerformanceMonitor, error) {
	if !c.Enabled {
		return monitoring.NewEmptyPerformanceMonitor(), nil
	}
	logger, err := createPerformanceMonitorLogger(c.LogFileSize)
	if err != nil {
		return monitoring.NewEmptyPerformanceMonitor(), err
	}
	interval, err := time.ParseDuration(c.Interval)
	if err != nil {
		return monitoring.NewEmptyPerformanceMonitor(), err
	}
	return monitoring.NewPerformanceMonitor(interval, logger), nil
}

func createPerformanceMonitorLogger(logFileSize int) (log.Logger, error) {
	l := log.New()
	logLvl := log.LvlInfo
	fileHandler, err := logUtil.GetLogFileHandler("performance.log", logFileSize)
	if err != nil {
		return nil, err
	}
	l.SetHandler(log.LvlFilterHandler(logLvl, fileHandler))
	return l, nil
}

type app struct {
	server api.Server
	db     db.Accessor
	logger log.Logger
}

func (e *app) Start(swaggerConfig config.SwaggerConfig) {
	e.server.Start(swaggerConfig)
}

func (e *app) Destroy() {
	e.db.Destroy()
}
