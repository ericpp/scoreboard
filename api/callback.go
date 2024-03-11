package handler

import (
  "fmt"
  "log"
  "net/http"
  "net/url"
  "strings"
  "io/ioutil"
  "os"
)

func CBSetAccessToken(stringToken string) error {
  setUrl := fmt.Sprintf("%s/set/authToken", os.Getenv("KV_REST_API_URL"))
log.Print(setUrl)
  client := &http.Client{}
  req, _ := http.NewRequest("POST", setUrl, strings.NewReader(stringToken))

  req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("KV_REST_API_TOKEN")))

  resp, _ := client.Do(req)

  body, err := ioutil.ReadAll(resp.Body)
  if err != nil {
    return err
  }

  log.Print(string(body))

  return nil
}


func Handler(w http.ResponseWriter, r *http.Request) {
  if r.FormValue("code") != "" {
    postUrl := "https://api.getalby.com/oauth/token"

    form := url.Values{}
    form.Add("code", r.FormValue("code"))
    form.Add("grant_type", "authorization_code")
    form.Add("redirect_uri", os.Getenv("ALBY_REDIRECT_URI"))
    form.Add("client_id", os.Getenv("ALBY_CLIENT_ID"))
    form.Add("client_secret", os.Getenv("ALBY_CLIENT_SECRET"))
    encoded := form.Encode()

    client := &http.Client{}

    req, _ := http.NewRequest("POST", postUrl, strings.NewReader(encoded))

    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.Header.Set("User-Agent", "Scoreboard")

    resp, _ := client.Do(req)

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
      fmt.Fprintf(w, "Err %%", resp)
    }

    sb := string(body)

    CBSetAccessToken(sb)

    fmt.Fprint(w, sb)
  }
}
