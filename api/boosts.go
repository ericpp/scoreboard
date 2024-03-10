package handler

import (
  "encoding/json"
  "fmt"
  "io/ioutil"
  "net/http"
  "net/url"
  "log"
  "strings"
  "os"
)

type IncomingBoost struct {
  Amount       float64      `json:"amount"`
  Boostagram   interface{}  `json:"boostagram"`
  CreatedAt    string       `json:"created_at"`
  CreationDate float64      `json:"creation_date"`
  Identifier   string       `json:"identifier"`
  Value        float64      `json:"value"`
}

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

  token := js["access_token"].(string)

  return token, nil
}

func GetAccessToken() (string, error) {
  body, err := os.ReadFile("/tmp/alby-access")

  if err == nil {
    return string(body), nil
  }

  token, nil := RequestAccessToken()

  os.WriteFile("/tmp/alby-access", []byte(token), 0644)

  return token, nil
}


func Handler(w http.ResponseWriter, r *http.Request) {
  client := &http.Client{}
  req, err := http.NewRequest("GET", "https://api.getalby.com/invoices/incoming", nil)

  if err != nil {
    log.Print(err)
    os.Exit(1)
  }

  token, err := GetAccessToken()

  if err != nil {
    log.Print(err)
    os.Exit(1)
  }

  req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
  req.Header.Set("User-Agent", "Scoreboard")

  q := req.URL.Query()

  if r.FormValue("page") != "" {
    q.Add("page", r.FormValue("page"))
  }

  if r.FormValue("items") != "" {
    q.Add("items", r.FormValue("items"))
  }

  if r.FormValue("since") != "" {
    q.Add("q[since]", r.FormValue("since"))
  }

  if r.FormValue("created_at_lt") != "" {
    q.Add("q[created_at_lt]", r.FormValue("created_at_lt"))
  }

  if r.FormValue("created_at_gt") != "" {
    q.Add("q[created_at_gt]", r.FormValue("created_at_gt"))
  }

  req.URL.RawQuery = q.Encode()

  resp, err := client.Do(req)

  if err != nil {
    log.Print(err)
    os.Exit(1)
  }

  body, err := ioutil.ReadAll(resp.Body)

  if err != nil {
    log.Print(err)
    os.Exit(1)
  }

  // sb := string(body)

  var transactions []map[string]interface{}

  if err := json.Unmarshal(body, &transactions); err != nil {
    log.Print(err)
    os.Exit(1)
  }

  boosts := []IncomingBoost{}

  for _, transaction := range transactions {
    if transaction["boostagram"] != nil {
      boosts = append(boosts, IncomingBoost {
        Amount: transaction["amount"].(float64),
        Boostagram: transaction["boostagram"],
        CreatedAt: transaction["created_at"].(string),
        CreationDate: transaction["creation_date"].(float64),
        Identifier: transaction["identifier"].(string),
        Value: transaction["value"].(float64),
      })
    }
  }

  w.Header().Set("Content-Type", "application/json; charset=utf-8")

  js, err := json.Marshal(boosts); 

  if err != nil {
    log.Print(err)
    os.Exit(1)
  }

  fmt.Fprint(w, string(js))

}
