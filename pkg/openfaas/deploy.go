package openfaas

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Function struct {
	Service     string            `json:"service"`
	Image       string            `json:"image"`
	EnvProcess  string            `json:"envProcess"`
	Annotations map[string]string `json:"annotations"`
}

func (c *Client) Deploy(f Function) error {
	do := func(method string) error {
		const endpoint = "/system/functions"

		b := &bytes.Buffer{}
		if err := json.NewEncoder(b).Encode(&f); err != nil {
			return err
		}

		req, err := http.NewRequest(
			method,
			c.Host+endpoint,
			b,
		)
		if err != nil {
			return err
		}
		req.Header.Add("Content-Type", "application/json")
		c.Auth(req)

		res, err := c.Client.Do(req)
		if err != nil {
			return err
		}
		if res.Body != nil {
			defer res.Body.Close()
		}

		switch res.StatusCode {
		case http.StatusOK:
			return nil
		case http.StatusAccepted:
			return nil
		case http.StatusBadRequest:
			return ErrBadRequest
		case http.StatusNotFound:
			return ErrNotFound
		case http.StatusInternalServerError:
			return ErrInternalServerError
		default:
			bodyBytes, err := ioutil.ReadAll(res.Body)
			if err != nil {
				return err
			}
			return fmt.Errorf(
				"%w: %d: %s",
				ErrUnknownResponse,
				res.StatusCode,
				string(bodyBytes),
			)
		}
	}

	err := do(http.MethodPut)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			err := do(http.MethodPost)
			if err != nil {
				return err
			}
		}
		return err
	}

	return nil
}
