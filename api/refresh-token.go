package handler

import (
  "encoding/json"
  "errors"
  "fmt"
  "io/ioutil"
  "net/http"
  "net/url"
  "log"
  "strings"
  "os"
)

type KVResult struct {
  Result           string  `json:"result"`
}

type AlbyToken struct {
  AccessToken      string  `json:"access_token"`
  ExpiresIn        float64 `json:"expires_in"`
  RefreshToken     string  `json:"refresh_token"`
  Scope            string  `json:"scope"`
  TokenType        string  `json:"token_type"`
  Error            string  `json:"error,omitempty"`
  ErrorDescription string  `json:"error_description,omitempty"`
}

func GetAccessToken() (*AlbyToken, error) {
  getUrl := fmt.Sprintf("%s/get/authToken", os.Getenv("KV_REST_API_URL"))

  client := &http.Client{}
  req, _ := http.NewRequest("GET", getUrl, nil)

  req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("KV_REST_API_TOKEN")))

  resp, _ := client.Do(req)

  body, err := ioutil.ReadAll(resp.Body)
  if err != nil {
    return nil, err
  }

  var result KVResult

  if err := json.Unmarshal(body, &result); err != nil {
    return nil, err
  }

  var token AlbyToken

  if err := json.Unmarshal([]byte(result.Result), &token); err != nil {
    return nil, err
  }

  return &token, nil
}

func SetAccessToken(token AlbyToken) error {
  setUrl := fmt.Sprintf("%s/set/authToken", os.Getenv("KV_REST_API_URL"))

  encoded, err := json.Marshal(token)

  if err != nil {
    log.Print(err)
    os.Exit(1)
  }

  client := &http.Client{}
  req, _ := http.NewRequest("POST", setUrl, strings.NewReader(string(encoded)))

  req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("KV_REST_API_TOKEN")))

  resp, _ := client.Do(req)

  body, err := ioutil.ReadAll(resp.Body)
  if err != nil {
    return err
  }

  log.Print(string(body))

  return nil
}

func RefreshAccessToken(currToken *AlbyToken) (*AlbyToken, error) {
  postUrl := "https://api.getalby.com/oauth/token"

  refToken := os.Getenv("ALBY_REFRESH_TOKEN")
  if currToken != nil {
    refToken = currToken.RefreshToken
  }

  form := url.Values{}
  form.Add("client_id", os.Getenv("ALBY_CLIENT_ID"))
  form.Add("client_secret", os.Getenv("ALBY_CLIENT_SECRET"))
  form.Add("grant_type", "refresh_token")
  form.Add("refresh_token", refToken)
  encoded := form.Encode()

  client := &http.Client{}
  req, _ := http.NewRequest("POST", postUrl, strings.NewReader(encoded))

  req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
  req.Header.Set("User-Agent", "Scoreboard")

  resp, _ := client.Do(req)

  body, err := ioutil.ReadAll(resp.Body)
  if err != nil {
    return nil, err
  }

  var token AlbyToken

  if err := json.Unmarshal(body, &token); err != nil {
    log.Print(body)
    return nil, err
  }

  if token.Error != "" {
    return nil, errors.New(token.ErrorDescription)
  }

  SetAccessToken(token)

  return &token, nil
}

func Handler(w http.ResponseWriter, r *http.Request) {

  token, err := GetAccessToken()

  if err != nil {
    log.Print(err)
    return
  }

  if token != nil {
    token, err = RefreshAccessToken(token)
    log.Print(token)
  }

  fmt.Fprint(w, "OK")

}
