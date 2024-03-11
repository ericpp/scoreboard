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

func RequestAccessToken() (string, error) {
  postUrl := "https://api.getalby.com/oauth/token"

  form := url.Values{}
  form.Add("client_id", os.Getenv("ALBY_CLIENT_ID"))
  form.Add("client_secret", os.Getenv("ALBY_CLIENT_SECRET"))
  form.Add("grant_type", "refresh_token")
  form.Add("refresh_token", os.Getenv("ALBY_REFRESH_TOKEN"))
  encoded := form.Encode()

  client := &http.Client{}
  req, _ := http.NewRequest("POST", postUrl, strings.NewReader(encoded))

  req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
  req.Header.Set("User-Agent", "Scoreboard")

  resp, _ := client.Do(req)

  body, err := ioutil.ReadAll(resp.Body)
  if err != nil {
    return "", err
  }

  var js map[string]interface{}

  if err := json.Unmarshal(body, &js); err != nil {
    return "", err
  }

  if js["error"] != nil {
    return "", errors.New(js["error_description"].(string))
  }

  token := js["access_token"].(string)

  return token, nil
}

func Handler(w http.ResponseWriter, r *http.Request) {
  token, err := RequestAccessToken()

  if err != nil {
    log.Print(err)
    fmt.Fprint(w, "Error refreshing access token")
    return
  }

  if len(token) != 48 {
    fmt.Fprint(w, "Invalid access token received")
    return
  }

  os.WriteFile("/tmp/alby-access", []byte(token), 0644)

  fmt.Fprint(w, "Access token refreshed")
}
