module github.com/idena-network/idena-indexer-api

go 1.13

replace github.com/cosmos/iavl => github.com/idena-network/iavl v0.12.3-0.20210112075003-70ccb13c86c9

require (
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751
	github.com/coreos/go-semver v0.3.0
	github.com/go-stack/stack v1.8.0
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/idena-network/idena-go v0.25.2
	github.com/klauspost/compress v1.11.13 // indirect
	github.com/lib/pq v1.10.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/shopspring/decimal v1.2.0
	github.com/stretchr/testify v1.7.0
	github.com/swaggo/http-swagger v1.0.0
	github.com/swaggo/swag v1.7.0
	github.com/valyala/fasthttp v1.23.0
	gopkg.in/urfave/cli.v1 v1.20.0
)
