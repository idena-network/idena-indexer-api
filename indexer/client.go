package indexer

import (
	"encoding/json"
	"fmt"
	"github.com/idena-network/idena-indexer-api/log"
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
	"strings"
	"time"
)

type Client interface {
	Get(query string, resultType interface{}) (interface{}, *string, error)
}

func NewClient(indexerUrl string, maxConnections int) Client {
	log.Info(fmt.Sprintf("Initializing indexer client, url: %v, max connections: %v", indexerUrl, maxConnections))
	res := &clientImpl{
		indexerUrl: indexerUrl,
		pool: &fasthttp.Client{
			MaxConnsPerHost:    maxConnections,
			MaxConnWaitTimeout: time.Second * 30,
		},
	}
	return res
}

type clientImpl struct {
	indexerUrl string
	pool       *fasthttp.Client
}

func (client *clientImpl) Get(query string, resultType interface{}) (interface{}, *string, error) {
	url := strings.Join([]string{client.indexerUrl, query}, "/")
	statusCode, body, err := client.pool.Get(nil, url)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "unable to send request %v", url)
	}
	if statusCode != fasthttp.StatusOK {
		return nil, nil, errors.New(fmt.Sprintf("unable to send request %v, status code: %v", url, statusCode))
	}
	resp := &response{
		Result: resultType,
	}
	if err := json.Unmarshal(body, resp); err != nil {
		return nil, nil, errors.Wrapf(err, "unable to parse response of %v", url)
	}
	if resp.Error != nil {
		return nil, nil, errors.New(fmt.Sprintf("got error response from %v: %v", url, resp.Error.Message))
	}
	return resp.Result, resp.ContinuationToken, nil
}

type response struct {
	Result            interface{} `json:"result,omitempty"`
	Error             *respError  `json:"error,omitempty"`
	ContinuationToken *string     `json:"continuationToken,omitempty"`
}

type respError struct {
	Message string `json:"message"`
}
