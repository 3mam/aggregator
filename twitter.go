package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"fmt"

	"github.com/ChimeraCoder/anaconda"
	"github.com/garyburd/go-oauth/oauth"
)

func twitter(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.RawQuery == "":
		twitterRoot(w, r)
	case strings.Contains(r.URL.RawQuery, "login"):
		twitterLogin(w, r)
	case strings.Contains(r.URL.RawQuery, "read"):
		if r.URL.RawQuery[:len("read")] == "read" {
			twitterRead(w, r)
		}
	}
}

func twitterRoot(w http.ResponseWriter, r *http.Request) {
	var htmlData string
	var db, _ = sql.Open("sqlite3", "./agregator.db")
	var rows, _ = db.Query("select twitt, id from twitts where read=0")
	var twitt string
	var list []int64
	var id int64
	for rows.Next() {
		rows.Scan(&twitt, &id)
		htmlData += twitt
		list = append(list, id)
	}
	rows.Close()
	db.Close()
	var jsonList, _ = json.Marshal(list)
	var buf = new(bytes.Buffer)
	var t, _ = template.New("twitter.html").ParseFiles("html/twitter.html")
	t.Execute(buf, &Page{"agregator", htmlData, string(jsonList)})
	w.Write(buf.Bytes())
}

var gUrl string
var gCre *oauth.Credentials

func twitterLogin(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery == "login" {
		gUrl, gCre, _ = anaconda.AuthorizationURL("http://localhost:8080/twitter?login")
		fmt.Println(r.URL.RawQuery)
		http.Redirect(w, r, gUrl, http.StatusFound)
	} else {
		var key = strings.Split(r.URL.RawQuery, "&")[2][15:]
		var cred, _, _ = anaconda.GetCredentials(gCre, key)
		//		db.Exec("insert or replace into twitter values('login', ?, ?)", cred.Token, cred.Secret)
		var db, _ = sql.Open("sqlite3", "./agregator.db")
		db.Exec("update login set token=?, secret=? where id='twitter'", cred.Token, cred.Secret)
		db.Close()
		http.Redirect(w, r, "/twitter", http.StatusFound)
	}

}

func twitterRead(w http.ResponseWriter, r *http.Request) {
	defer http.Redirect(w, r, "/twitter", http.StatusFound)
	var list []int64
	var db, _ = sql.Open("sqlite3", "./agregator.db")
	var tx, _ = db.Begin()
	if json.Unmarshal([]byte(r.URL.RawQuery[len("read="):]), &list) != nil {
		return
	}
	var stmt, _ = tx.Prepare("update twitts set read=1 where id=?")
	for i := range list {
		stmt.Exec(list[i])
	}
	stmt.Close()
	tx.Commit()
	db.Close()
}

//Delete from items where read=1 and rowid not IN (Select rowid from items limit 1000);
//delete from Table where id > 79 and id < 296

func twitterUpdate(db *sql.DB) {
	var stmt, _ = db.Prepare("select token, secret from login where id = ?")
	var token string
	var secret string
	stmt.QueryRow("twitter").Scan(&token, &secret)
	stmt.Close()
	var api = anaconda.NewTwitterApi(token, secret)
	var v = url.Values{}
	v.Set("count", "200")
	v.Set("include_entities", "false")
	v.Set("trim_user", "false")
	v.Set("exclude_replies", "false")
	var t, _ = api.GetHomeTimeline(v)
	var id string
	id += "<html>" + "<meta name=\"twitter:widgets:theme\" content=\"dark\">" + "<script async src=\"//platform.twitter.com/widgets.js\" charset=\"utf-8\"></script>"
	var tx, _ = db.Begin()
	stmt, _ = tx.Prepare("insert or ignore into twitts(id, twitt, time, read) values(?,?,?,0)")
	for i := len(t) - 1; -1 < i; i-- {
		if dateToUnix(t[i].CreatedAt) > time.Now().AddDate(0, 0, TIME_DELETE).Unix() {
			var h, _ = api.GetOEmbedId(t[i].Id, nil)
			var twitt = strings.Replace(h.Html, "<script async src=\"//platform.twitter.com/widgets.js\" charset=\"utf-8\"></script>", "", 1)
			stmt.Exec(t[i].IdStr, twitt, dateToUnix(t[i].CreatedAt))
		}
	}
	stmt.Close()
	tx.Commit()
}
