package model

import (
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	ID           int    `gorm:"unique;primaryKey;not null"`
	TgId         int64  `gorm:"not null"`
	RefreshToken string `gorm:"not null"`
	MsId         string `gorm:"not null"`
	Uptime       int64  `gorm:"autoUpdateTime;not null"`
	Alias        string `gorm:"not null"`
	ClientId     string `gorm:"not null"`
	ClientSecret string `gorm:"not null"`
	Other        string
}
type ErrClient struct {
	*Client
	Err error
}

const (
	msApiUrl    string = "https://login.microsoftonline.com"
	msGraUrl    string = "https://graph.microsoft.com"
	redirectUri string = "http://localhost/e5sub"
	scope       string = "openid offline_access mail.read user.read"
)

var client = &http.Client{}

func init() {
	client.Timeout = 10 * time.Second
	tp := http.DefaultTransport.(*http.Transport).Clone()
	//https://gocn.vip/topics/11970
	//DefaultMaxIdleConnsPerHost 设置的太小就会导致一个问题,
	//在大量请求的情况下去访问特定的 host 的时候,长连接会退化成短链接.
	tp.MaxIdleConns = 0
	tp.TLSHandshakeTimeout = 20 * time.Second
	tp.MaxIdleConnsPerHost = 50
	tp.ResponseHeaderTimeout = 20 * time.Second
	//to avoid "context deadline exceeded (Client.Timeout exceeded while awaiting headers)"
	//https://cloud.tencent.com/developer/article/1529840
	tp.IdleConnTimeout = 20 * time.Second
	tp.ExpectContinueTimeout = 20 * time.Second

	client.Transport = tp
}
func NewClient(clientId string, clientSecret string) *Client {
	return &Client{
		ClientId:     clientId,
		ClientSecret: clientSecret,
	}
}
func GetMSAuthUrl(clientId string) string {
	return "https://login.microsoftonline.com/common/oauth2/v2.0/authorize?client_id=" + clientId + "&response_type=code&redirect_uri=" + url.QueryEscape(redirectUri) + "&response_mode=query&scope=" + url.QueryEscape(scope)
}
func GetMSRegisterAppUrl() string {
	ru := "https://developer.microsoft.com/en-us/graph/quick-start?appID=_appId_&appName=_appName_&redirectUrl=http://localhost:8000&platform=option-windowsuniversal"
	deeplink := "/quickstart/graphIO?publicClientSupport=false&appName=e5sub&redirectUrl=http://localhost/e5sub&allowImplicitFlow=false&ru=" + url.QueryEscape(ru)
	appUrl := "https://apps.dev.microsoft.com/?deepLink=" + url.QueryEscape(deeplink)
	return appUrl
}

// GetTokenWithCode return access_token and refresh_token
func (c *Client) GetTokenWithCode(code string) (error error) {
	var r http.Request
	r.ParseForm()
	r.Form.Add("client_id", c.ClientId)
	r.Form.Add("client_secret", c.ClientSecret)
	r.Form.Add("grant_type", "authorization_code")
	r.Form.Add("scope", scope)
	r.Form.Add("code", code)
	r.Form.Add("redirect_uri", redirectUri)
	body := strings.NewReader(r.Form.Encode())
	req, err := http.NewRequest("POST", msApiUrl+"/common/oauth2/v2.0/token", body)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return err
	}
	if gjson.Get(string(content), "token_type").String() == "Bearer" {
		c.RefreshToken = gjson.Get(string(content), "refresh_token").String()
		return nil
	}
	return errors.New(string(content))
}

//return access_token and new refresh token
func (c *Client) getToken() (accessToken string, error error) {
	var r http.Request
	r.ParseForm()
	r.Form.Add("client_id", c.ClientId)
	r.Form.Add("client_secret", c.ClientSecret)
	r.Form.Add("grant_type", "refresh_token")
	r.Form.Add("scope", scope)
	r.Form.Add("refresh_token", c.RefreshToken)
	r.Form.Add("redirect_uri", redirectUri)
	body := strings.NewReader(r.Form.Encode())
	//fmt.Println(body)
	req, err := http.NewRequest("POST", msApiUrl+"/common/oauth2/v2.0/token", body)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if gjson.Get(string(content), "token_type").String() == "Bearer" {
		c.RefreshToken = gjson.Get(string(content), "refresh_token").String()
		return gjson.Get(string(content), "access_token").String(), nil
	}
	return "", errors.New(gjson.Get(string(content), "error").String())
}

// GetUserInfo Get User's Information
func (c *Client) GetUserInfo() (json string, error error) {
	var accessToken string
	req, err := http.NewRequest("GET", msGraUrl+"/v1.0/me", nil)
	if err != nil {
		return "", err
	}
	accessToken, err = c.getToken()
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", accessToken)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return "", err
	}
	if gjson.Get(string(content), "id").String() != "" {
		//fmt.Println("UserName: " + gjson.Get(string(content), "displayName").String())
		return string(content), nil
	}
	return "", errors.New(string(content))
}

func (c *Client) GetOutlookMails() error {
	var accessToken string
	req, err := http.NewRequest("GET", msGraUrl+"/v1.0/me/messages", nil)
	if err != nil {
		return err
	}
	accessToken, err = c.getToken()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", accessToken)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	//这里的.需要转义，否则会按路径的方式解析
	if gjson.Get(string(content), "@odata\\.context").String() != "" {
		return nil
	}
	return errors.New(gjson.Get(string(content), "error").String())
}
