package anticaptcha

import (
	"bytes"
	"encoding/json"
	"github.com/pkg/errors"
	"net/http"
	"net/url"
	"time"
)

var (
	baseURL      = &url.URL{Host: "api.anti-captcha.com", Scheme: "https", Path: "/"}
	sendInterval = 10 * time.Second
)

type Client struct {
	APIKey string
	c      *http.Client
}

func NewClient(api string) *Client {
	return &Client{
		APIKey: api,
		c:      &http.Client{Timeout: time.Minute},
	}
}

func (this *Client) UseDebug() {
	proxyURL, _ := url.Parse("http://localhost:8888")
	this.c.Transport = &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}
}

// Method to create the task to process the recaptcha, returns the task_id
func (this *Client) createTaskRecaptcha(websiteURL string, recaptchaKey string) (float64, error) {
	// Mount the data to be sent
	body := map[string]interface{}{
		"clientKey": this.APIKey,
		"task": map[string]interface{}{
			"type":       "NoCaptchaTaskProxyless",
			"websiteURL": websiteURL,
			"websiteKey": recaptchaKey,
		},
	}

	b, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}

	// Make the request
	u := baseURL.ResolveReference(&url.URL{Path: "/createTask"})
	resp, err := this.c.Post(u.String(), "application/json", bytes.NewBuffer(b))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Decode response
	responseBody := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&responseBody)
	if err != nil {
		return 0, err
	}

	taskId, ok := responseBody["taskId"]
	if ok {
		return taskId.(float64), nil
	}

	_, ok = responseBody["errorId"]
	if !ok {
		return 0, errors.New("anti-captcha: unknown response")
	}

	errorDescription, ok := responseBody["errorDescription"]
	if !ok {
		return 0, errors.New("anti-captcha: unknown error")
	}

	return 0, errors.New(errorDescription.(string))
}

// Method to check the result of a given task, returns the json returned from the api
func (this *Client) getTaskResult(taskID float64) (map[string]interface{}, error) {
	// Mount the data to be sent
	body := map[string]interface{}{
		"clientKey": this.APIKey,
		"taskId":    taskID,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	// Make the request
	u := baseURL.ResolveReference(&url.URL{Path: "/getTaskResult"})
	resp, err := this.c.Post(u.String(), "application/json", bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Decode response
	responseBody := make(map[string]interface{})
	json.NewDecoder(resp.Body).Decode(&responseBody)
	return responseBody, nil
}

// SendRecaptcha Method to encapsulate the processing of the recaptcha
// Given a url and a key, it sends to the api and waits until
// the processing is complete to return the evaluated key
func (this *Client) SendRecaptcha(websiteURL string, recaptchaKey string) (string, error) {
	// Create the task on anti-captcha api and get the task_id
	taskID, err := this.createTaskRecaptcha(websiteURL, recaptchaKey)
	if err != nil {
		return "", err
	}

	// Check if the result is ready, if not loop until it is
	response, err := this.getTaskResult(taskID)
	if err != nil {
		return "", err
	}
	for {
		if response["status"] == "processing" {
			//log.Println("Result is not ready, waiting a few seconds to check again...")
			time.Sleep(sendInterval)
			response, err = this.getTaskResult(taskID)
			if err != nil {
				return "", err
			}
		} else {
			//log.Println("Result is ready.")
			break
		}
	}

	if response["solution"] == nil {
		return "", errors.New("solution is null")
	}
	return response["solution"].(map[string]interface{})["gRecaptchaResponse"].(string), nil
}

// Method to create the task to process the image captcha, returns the task_id
func (this *Client) createTaskImage(imgString string) (float64, error) {
	// Mount the data to be sent
	body := map[string]interface{}{
		"clientKey": this.APIKey,
		"task": map[string]interface{}{
			"type": "ImageToTextTask",
			"body": imgString,
		},
	}

	b, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}

	// Make the request
	u := baseURL.ResolveReference(&url.URL{Path: "/createTask"})
	resp, err := this.c.Post(u.String(), "application/json", bytes.NewBuffer(b))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Decode response
	responseBody := make(map[string]interface{})
	if err = json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		return 0, err
	}

	val, ok := responseBody["taskId"]
	if !ok || val == nil {
		//fmt.Printf("%+v\n", responseBody)
		return 0, errors.New("anticaptcha error")
	}

	switch val.(type) {
	case float64:
		return responseBody["taskId"].(float64), nil
	default:
		//fmt.Printf("%+v\n", responseBody)
		return 0, errors.New("anticaptcha error")
	}
}

// SendImage Method to encapsulate the processing of the image captcha
// Given a base64 string from the image, it sends to the api and waits until
// the processing is complete to return the evaluated key
func (this *Client) SendImage(imgString string) (string, error) {
	// Create the task on anti-captcha api and get the task_id
	taskID, err := this.createTaskImage(imgString)
	if err != nil {
		return "", err
	}

	// Check if the result is ready, if not loop until it is
	response, err := this.getTaskResult(taskID)
	if err != nil {
		return "", err
	}
	for {
		if response["status"] == "processing" {
			//log.Println("Result is not ready, waiting a few seconds to check again...")
			time.Sleep(sendInterval)
			response, err = this.getTaskResult(taskID)
			if err != nil {
				return "", err
			}
		} else {
			//log.Println("Result is ready.")
			break
		}
	}

	if response["solution"] == nil {
		return "", errors.New("anticaptcha error")
	}

	return response["solution"].(map[string]interface{})["text"].(string), nil
}
