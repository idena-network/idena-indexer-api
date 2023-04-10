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
	Post(url string, body []byte) (userErr, err error)
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

func (client *clientImpl) Post(url string, body []byte) (userErr, err error) {
	url = strings.Join([]string{client.indexerUrl, url}, "/")
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(url)
	req.Header.SetMethod("POST")
	req.SetBody(body)
	resp := fasthttp.AcquireResponse()
	if err := client.pool.Do(req, resp); err != nil {
		return nil, errors.Wrapf(err, "unable to send request %v", url)
	}
	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, errors.New(fmt.Sprintf("unable to send request %v, status code: %v", url, resp.StatusCode()))
	}
	respBody := &response{
		Error: new(respError),
	}
	if err := json.Unmarshal(resp.Body(), respBody); err != nil {
		return nil, errors.Wrapf(err, "unable to parse response of %v", url)
	}
	if respBody.Error != nil {
		if len(respBody.Error.UserMessage) > 0 {
			return errors.New(respBody.Error.UserMessage), nil
		}
		if len(respBody.Error.Message) > 0 {
			return nil, errors.New(fmt.Sprintf("got error response from %v: %v", url, respBody.Error.Message))
		}
	}
	return nil, nil
}

type response struct {
	Result            interface{} `json:"result,omitempty"`
	Error             *respError  `json:"error,omitempty"`
	ContinuationToken *string     `json:"continuationToken,omitempty"`
}

type respError struct {
	UserMessage string `json:"userMessage"`
	Message     string `json:"message"`
}
