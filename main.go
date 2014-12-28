package jira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Error struct {
	StatusCode int
	Status     string
	Message    string
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Status, e.Message)
}

type Issue struct {
	Id      string
	Key     string
	Summary string
	Project string
	Data    map[string]interface{}
}

type Jira struct {
	baseUrl *url.URL
	user    string
	pass    string
	res     *http.Client
}

func New(jiraUrl string, user string, pass string, timeout time.Duration) (
	*Jira, error) {
	baseUrl, err := url.Parse(jiraUrl)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Transport: &http.Transport{
		Dial: func(proto, addr string) (net.Conn, error) {
			return net.DialTimeout(proto, addr, timeout)
		},
	}}

	jira := &Jira{
		baseUrl: baseUrl,
		user:    user,
		pass:    pass,
		res:     httpClient,
	}

	return jira, nil
}

func (jira *Jira) GetIssue(key string, fields []string) (
	issue *Issue, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	response, err := jira.Request("GET",
		"issue/"+key+"/?fields="+strings.Join(fields, ","),
		[]byte{})
	if err != nil {
		return nil, err
	}

	rawData := map[string]interface{}{}

	if err := json.Unmarshal(response, &rawData); err != nil {
		return nil, err
	}

	issue = &Issue{
		Id:  rawData["id"].(string),
		Key: rawData["key"].(string),
	}
	tmp := strings.Split(issue.Key, "-")
	issue.Project = strings.ToLower(tmp[0])
	issue.Data = rawData["fields"].(map[string]interface{})

	if summary, ok := issue.Data["summary"].(string); ok {
		issue.Summary = summary
	}

	return issue, nil
}

func (jira *Jira) GetProjectTitle(key string) (title string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()
	body, err := jira.Request("GET", "project/"+key, []byte{})
	if err != nil {
		return "", err
	}
	var rawData interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return "", err
	}
	return rawData.(map[string]interface{})["name"].(string), nil
}

func (jira *Jira) Comment(issue string, msg string) error {
	type comment struct {
		Data string `json:"body"`
	}

	body, err := json.Marshal(comment{Data: msg})
	if err != nil {
		return err
	}
	_, err = jira.Request("POST", "issue/"+issue+"/comment", body)
	if err != nil {
		return err
	}

	return nil
}

func (jira *Jira) Request(method string, path string, body []byte) (
	[]byte, error) {
	buffer := bytes.NewBuffer(body)

	req, err := http.NewRequest(method, jira.baseUrl.String()+path, buffer)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json; charset=utf-8")
	req.SetBasicAuth(jira.user, jira.pass)

	resp, err := jira.res.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		return nil, Error{
			StatusCode: resp.StatusCode, Status: resp.Status,
			Message: "Not Found"}
	}

	if resp.StatusCode >= 500 {
		return nil, Error{StatusCode: resp.StatusCode,
			Status: resp.Status, Message: string(data)}
	}

	return data, nil
}
