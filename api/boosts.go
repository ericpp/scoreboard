package handler

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "net/http"
    "log"
    "strconv"
    "strings"
    "os"
    _ "github.com/lib/pq"
)

type IncomingBoost struct {
    Amount           float64      `json:"amount"`
    Boostagram       interface{}  `json:"boostagram"`
    CreatedAt        string       `json:"created_at"`
    CreationDate     float64      `json:"creation_date"`
    Identifier       string       `json:"identifier"`
    Value            float64      `json:"value"`
}

func GetBoosts(query map[string]string) ([]IncomingBoost, error) {
    // open database
    db, err := sql.Open("postgres", os.Getenv("POSTGRES_URL"))

    if err != nil {
        return nil, err
    }

    // close database
    defer db.Close()

    // check db
    if err = db.Ping(); err != nil {
        return nil, err
    }

    var where []string
    var params []any

    items := 25
    offset := 0

    if val, ok := query["q[created_at_lt]"]; ok {
        params = append(params, val)
        where = append(where, fmt.Sprintf(`creation_date <= $%d`, len(params)))
    }

    if val, ok := query["q[created_at_gt]"]; ok {
        params = append(params, val)
        where = append(where, fmt.Sprintf(`creation_date >= $%d`, len(params)))
    }

    if val, ok := query["q[since]"]; ok {
        params = append(params, val)
        where = append(where, fmt.Sprintf(`creation_date >= (SELECT MAX(creation_date) FROM invoices WHERE identifier = $%d)`, len(params)))

        params = append(params, val)
        where = append(where, fmt.Sprintf(`identifier <> $%d`, len(params)))
    }

    podcast, hasPodcast := query["q[podcast]"]
    eventGuid, hasEventGuid := query["q[eventGuid]"]
    episodeGuid, hasEpisodeGuid := query["q[episodeGuid]"]
    placeholders := []string{}

    if hasPodcast {
        for _, val := range strings.Split(podcast, ",") {
            if val == "" {
                continue
            }
            params = append(params, "%"+val+"%")
            placeholders = append(placeholders, fmt.Sprintf(`podcast ILIKE $%d`, len(params)))
        }
    }

    if hasEventGuid {
        for _, val := range strings.Split(eventGuid, ",") {
            if val == "" {
                continue
            }
            params = append(params, val)
            placeholders = append(placeholders, fmt.Sprintf(`event_guid = $%d`, len(params)))
        }
    }

    if hasEpisodeGuid {
        for _, val := range strings.Split(episodeGuid, ",") {
            if val == "" {
                continue
            }
            params = append(params, val)
            placeholders = append(placeholders, fmt.Sprintf(`episode_guid = $%d`, len(params)))
        }
    }

    if len(placeholders) > 0 {
        where = append(where, fmt.Sprintf(`(%s)`, strings.Join(placeholders, " OR ")))
    }

    if val, ok := query["items"]; ok {
        num, err := strconv.Atoi(val)
        if err != nil {
            return nil, err
        }

        items = num
    }

    if val, ok := query["page"]; ok {
        pg, err := strconv.Atoi(val)
        if err != nil {
            return nil, err
        }

        offset = (pg - 1) * items
    }

    if len(where) == 0 {
        where = append(where, "1=1")
    }

    sql := fmt.Sprintf(`SELECT amount, boostagram, created_at, creation_date, identifier, value FROM invoices WHERE %s ORDER BY creation_date DESC LIMIT %d OFFSET %d`, strings.Join(where, " AND "), items, offset)

    rows, err := db.Query(sql, params...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    boosts := []IncomingBoost{}

    // Loop through rows, using Scan to assign column data to struct fields.
    for rows.Next() {
        var item IncomingBoost
        var boostagram string

        if err := rows.Scan(&item.Amount, &boostagram, &item.CreatedAt, &item.CreationDate, &item.Identifier, &item.Value); err != nil {
            return nil, err
        }

        if err := json.Unmarshal([]byte(boostagram), &item.Boostagram); err != nil {
            return nil, err
        }

        boosts = append(boosts, item)
    }

    if err = rows.Err(); err != nil {
        return nil, err
    }

    return boosts, nil
}

func Handler(w http.ResponseWriter, r *http.Request) {
    // Parse the form data to populate r.Form
    if err := r.ParseForm(); err != nil {
        http.Error(w, "Error parsing form data", http.StatusBadRequest)
        return
    }

    query := make(map[string]string)

    if r.FormValue("page") != "" {
        query["page"] = r.FormValue("page")
    }

    if r.FormValue("items") != "" {
        query["items"] = r.FormValue("items")
    }

    if r.FormValue("since") != "" {
        query["q[since]"] = r.FormValue("since")
    }

    if r.FormValue("created_at_lt") != "" {
        query["q[created_at_lt]"] = r.FormValue("created_at_lt")
    }

    if r.FormValue("created_at_gt") != "" {
        query["q[created_at_gt]"] = r.FormValue("created_at_gt")
    }

    // Handle multiple podcast values
    if len(r.Form["podcast"]) > 0 {
        if len(r.Form["podcast"]) == 1 {
            query["q[podcast]"] = r.Form["podcast"][0]
        } else {
            // Join multiple values with comma
            query["q[podcast]"] = strings.Join(r.Form["podcast"], ",")
        }
    }

    // Handle multiple eventGuid values
    if len(r.Form["eventGuid"]) > 0 {
        if len(r.Form["eventGuid"]) == 1 {
            query["q[eventGuid]"] = r.Form["eventGuid"][0]
        } else {
            // Join multiple values with comma
            query["q[eventGuid]"] = strings.Join(r.Form["eventGuid"], ",")
        }
    }

    // Handle multiple episodeGuid values
    if len(r.Form["episodeGuid"]) > 0 {
        if len(r.Form["episodeGuid"]) == 1 {
            query["q[episodeGuid]"] = r.Form["episodeGuid"][0]
        } else {
            // Join multiple values with comma
            query["q[episodeGuid]"] = strings.Join(r.Form["episodeGuid"], ",")
        }
    }

    boosts, err := GetBoosts(query)
    if err != nil {
        log.Fatal(err)
    }

    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.Header().Set("Access-Control-Allow-Origin", "*")

    js, err := json.Marshal(boosts);

    if err != nil {
        log.Print(err)
        os.Exit(1)
    }

    fmt.Fprint(w, string(js))
}
