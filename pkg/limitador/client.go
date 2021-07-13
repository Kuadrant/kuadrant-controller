/*
 Copyright 2021 Red Hat, Inc.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package limitador

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kuadrant/kuadrant-controller/pkg/common"
)

// TODO(eastizle): Use the limitador operator to manage rate limits in limitador service

type RateLimit struct {
	Conditions []string `json:"conditions"`
	MaxValue   int      `json:"max_value"`
	Namespace  string   `json:"namespace"`
	Seconds    int      `json:"seconds"`
	Variables  []string `json:"variables"`
}

type Error struct {
	code int
	err  string
}

func (l Error) Error() string {
	return fmt.Sprintf("error calling limitador - reason: %s - http statuscode: %d", l.err, l.code)
}

func limitadorError(resp *http.Response) Error {
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Error{
			code: resp.StatusCode,
			err:  err.Error(),
		}
	}

	return Error{
		code: resp.StatusCode,
		err:  string(body),
	}
}

type Client struct {
	url        url.URL
	httpClient *http.Client
	logger     logr.Logger
}

func NewClient(url url.URL) Client {
	var transport http.RoundTripper
	if ctrl.Log.V(1).Enabled() {
		transport = &common.VerboseTransport{}
	}

	return Client{
		url:        url,
		httpClient: &http.Client{Transport: transport},
		logger:     ctrl.Log.WithName("limitador").WithName("client"),
	}
}

func (client *Client) CreateLimit(rateLimit RateLimit) error {
	jsonLimit, err := json.Marshal(rateLimit)
	if err != nil {
		return err
	}

	resp, err := client.httpClient.Post(
		fmt.Sprintf("%s/limits", client.url.String()),
		"application/json",
		bytes.NewBuffer(jsonLimit),
	)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return limitadorError(resp)
	}

	return err
}

func (client *Client) GetLimits(ns string) ([]RateLimit, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/limits/%s", client.url.String(), ns), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, limitadorError(resp)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	limits := []RateLimit{}
	if err := json.Unmarshal(body, &limits); err != nil {
		return nil, err
	}

	return limits, nil
}

func (client *Client) DeleteLimit(rateLimit RateLimit) error {
	jsonLimit, err := json.Marshal(rateLimit)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(
		"DELETE",
		fmt.Sprintf("%s/limits", client.url.String()),
		bytes.NewBuffer(jsonLimit),
	)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return limitadorError(resp)
	}

	return nil
}
