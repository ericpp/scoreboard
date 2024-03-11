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

type AlbyToken struct {
  AccessToken      string  `json:"access_token"`
  ExpiresIn        float64 `json:"expires_in"`
  RefreshToken     string  `json:"refresh_token"`
  Scope            string  `json:"scope"`
  TokenType        string  `json:"token_type"`
  Error            string  `json:"error,omitempty"`
  ErrorDescription string  `json:"error_description,omitempty"`
}

type IncomingBoost struct {
  Amount       float64      `json:"amount"`
  Boostagram   interface{}  `json:"boostagram"`
  CreatedAt    string       `json:"created_at"`
  CreationDate float64      `json:"creation_date"`
  Identifier   string       `json:"identifier"`
  Value        float64      `json:"value"`
}

func GetAccessToken() (*AlbyToken, error) {
  body, err := os.ReadFile("/tmp/alby-token")

  if os.IsNotExist(err) {
    return nil, nil
  }

  if err != nil {
    return nil, err
  }

  var token AlbyToken

  if err := json.Unmarshal(body, &token); err != nil {
    return nil, err
  }

  return &token, nil
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
    return nil, err
  }

  if token.Error != "" {
    return nil, errors.New(token.ErrorDescription)
  }

  os.WriteFile("/tmp/alby-token", []byte(body), 0644)

  return &token, nil
}

func GetTransactions(token AlbyToken, query map[string]string) (string, error) {
  client := &http.Client{}
  req, err := http.NewRequest("GET", "https://api.getalby.com/invoices/incoming", nil)

  if err != nil {
    log.Print(err)
    os.Exit(1)
  }

  req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
  req.Header.Set("User-Agent", "Scoreboard")

  q := req.URL.Query()

  for key, value := range query {
    q.Add(key, value)
  }


  req.URL.RawQuery = q.Encode()

  resp, err := client.Do(req)

  if err != nil {
    log.Print(err)
    return "", err
  }

  body, err := ioutil.ReadAll(resp.Body)

  if err != nil {
    log.Print(err)
    return "", err
  }

  return string(body), nil
}

func Handler(w http.ResponseWriter, r *http.Request) {

  query := make(map[string]string)

  if r.FormValue("page") != "" {
    query["page"] = "page"
  }

  if r.FormValue("items") != "" {
    query["items"] = "items"
  }

  if r.FormValue("since") != "" {
    query["q[since]"] = "since"
  }

  if r.FormValue("created_at_lt") != "" {
    query["q[created_at_lt]"] = "created_at_lt"
  }

  if r.FormValue("created_at_gt") != "" {
    query["q[created_at_gt]"] = "created_at_gt"
  }

  token, err := GetAccessToken()
  if err != nil {
    log.Print(err)
    os.Exit(1)
  }

  if token == nil {
    token, err = RefreshAccessToken(nil)

    if err != nil {
      log.Print(err)
      os.Exit(1)
    }
  }

  body, err := GetTransactions(*token, query)
  if err != nil {
    log.Print(err)
    os.Exit(1)
  }

  if strings.Contains(body, "invalid access token") {
    token, err = RefreshAccessToken(token)
    if err != nil {
      log.Print(err)
      os.Exit(1)
    }

    body, err = GetTransactions(*token, query)
    if err != nil {
      log.Print(err)
      os.Exit(1)
    }
  }

  var transactions []map[string]interface{}
log.Print(body)

  if err := json.Unmarshal([]byte(body), &transactions); err != nil {
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
