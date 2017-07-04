package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/Smartling/api-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const (
	codeSuccess = "SUCCESS"
)

type MainSuite struct {
	suite.Suite

	Mock struct {
		Server  *httptest.Server
		Request *http.Request
		Body    []byte
		Handler http.HandlerFunc
	}
}

func (suite *MainSuite) SetupSuite() {
	suite.Mock.Server = httptest.NewTLSServer(
		http.HandlerFunc(
			func(writer http.ResponseWriter, request *http.Request) {
				auth, err := handleAuthentication(writer, request)
				if err != nil {
					panic(err)
				}

				if auth {
					return
				}

				body, err := ioutil.ReadAll(request.Body)
				if err != nil {
					panic(err)
				}

				suite.Mock.Body = body
				suite.Mock.Request = request

				if suite.Mock.Handler != nil {
					suite.Mock.Handler(writer, request)
				}
			},
		),
	)
}

func (suite *MainSuite) TearDownSuite() {
	suite.Mock.Server.Close()
}

func (suite *MainSuite) run(opts ...interface{}) (bool, string, string) {
	var (
		success = true
		stdout  = &bytes.Buffer{}
		stderr  = &bytes.Buffer{}
	)

	args := []string{
		"-test.run=Test_Run",
		"--",
		"--insecure",
		"--smartling-url",
		suite.Mock.Server.URL,
	}

	for _, opt := range opts {
		switch opt := opt.(type) {
		case string:
			args = append(args, opt)
		}
	}

	cmd := exec.Command(
		os.Args[0],
		args...,
	)

	cmd.Env = append(cmd.Env, "_TEST_RUN=1")
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	if err, ok := err.(*exec.ExitError); ok {
		success = err.Success()
	}

	stdoutString := strings.TrimSuffix(stdout.String(), "PASS\n")
	stderrString := stderr.String()

	return success, stdoutString, stderrString
}

func (suite *MainSuite) assertStdout(output []string, args ...interface{}) {
	success, stdout, stderr := suite.run(args...)

	assert.True(suite.T(), success)
	assert.Empty(suite.T(), stderr)
	assert.Equal(
		suite.T(),
		strings.Join(output, "\n"),
		strings.TrimSpace(stdout),
	)
}

func TestMainSuite(t *testing.T) {
	suite.Run(t, &MainSuite{})
}

func Test_Run(t *testing.T) {
	if os.Getenv("_TEST_RUN") != "1" {
		return
	}

	var (
		skip = true
		args []string
	)

	for _, arg := range os.Args {
		if !skip {
			args = append(args, arg)
		}

		if arg == "--" {
			skip = false
		}
	}

	os.Args = []string{os.Args[0]}
	os.Args = append(os.Args, args...)

	main()
}

func writeSmartlingReply(
	writer io.Writer,
	code string,
	reply interface{},
) error {
	var payload struct {
		Response struct {
			Code string
			Data interface{}
		}
	}

	payload.Response.Code = code
	payload.Response.Data = reply

	return json.NewEncoder(writer).Encode(payload)
}

func handleAuthentication(
	writer http.ResponseWriter,
	request *http.Request,
) (bool, error) {
	if !strings.HasSuffix(request.URL.Path, "/authenticate") {
		return false, nil
	}

	writer.WriteHeader(http.StatusOK)

	err := writeSmartlingReply(
		writer,
		codeSuccess,
		struct {
			AccessToken      string
			ExpiresIn        int
			RefreshExpiresIn int
		}{
			"somenewtoken",
			480,
			3660,
		},
	)

	if err != nil {
		return false, err
	}

	return true, err
}

func utc(
	timestamp string,
) smartling.UTC {
	var utc smartling.UTC

	data, _ := json.Marshal(timestamp)

	err := utc.UnmarshalJSON([]byte(data))
	if err != nil {
		panic(err)
	}

	return utc
}