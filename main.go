package main

import (
	"database/sql"
	"log"
	"net/http"
	"time"
	"math/rand"
	"github.com/ChimeraCoder/anaconda"
	_ "github.com/mattn/go-sqlite3"
)

const TIME_UPDATE = time.Minute * 60
const TIME_DELETE = -4

type Page struct {
	Title string
	Data  string
	List  string
}

type NodeList struct {
	Parent int64
	Title  string
	Color  string
}

var gNodeList = make(map[int64]NodeList)

func main() {
	//	os.Remove("./agregator.db")

	var err error
	var db *sql.DB
	db, err = sql.Open("sqlite3", "./agregator.db")
	if err != nil {
		log.Fatal(err)
	}

	/*
	   	db.Exec(`
	   pragma main.page_size = 4096;
	   pragma main.temp_store = MEMORY;
	   pragma main.synchronous = NORMAL;
	   pragma main.journal_mode = OFF;
	   pragma main.cache_size = 5000;
	   	`)
	*/

	db.Exec("create table if not exists rss(node integer, parent integer, title text, type text, xmlurl text, color text)")
	db.Exec("create table if not exists items(id integer, time integer, parent integer, title text, url text primary key, read integer)")
	db.Exec("create table if not exists login(id text primary key, token text, secret text)")
	db.Exec("insert or ignore into login(id) values('twitter')")
	db.Exec("create table if not exists twitts(id text primary key, time integer, twitt text, read integer)")
	db.Close()
	anaconda.SetConsumerKey("UKzOWenp3H8PtynGFtLpvbHG3")
	anaconda.SetConsumerSecret("fGxGQBrzZd4fCe48TsdAU1CLqrgXc3wyrVnOMXBW47UsBK4q1p")
	go update()

	http.HandleFunc("/", index)
	http.HandleFunc("/rss", rss)
	http.HandleFunc("/twitter", twitter)
	log.Fatal(http.ListenAndServe(":8181", nil))
}

func index(w http.ResponseWriter, r *http.Request) {
}

func update() {
	for {
		rand.Seed(time.Now().UnixNano())
		//delete from Table where id > 79 and id < 296
		var db, _ = sql.Open("sqlite3", "./agregator.db")
		var date = time.Now().AddDate(0, 0, TIME_DELETE).Unix()
		db.Exec("delete from items where time < ?", date)
		db.Exec("delete from twitts where time < ?", date)
		rssUpdate(db)
		twitterUpdate(db)
		db.Close()
		time.Sleep(TIME_UPDATE)
	}
}

func dateToUnix(date string) int64 {
	var t time.Time
	var err error
	t, err = time.Parse("2006-01-02T15:04:05Z07:00", date)
	if err == nil {
		return t.Unix()
	}
	t, err = time.Parse("Mon, 02 Jan 2006 15:04:05 MST", date)
	if err == nil {
		return t.Unix()
	}
	t, err = time.Parse("Mon, 02 Jan 2006 15:04:05 -0700", date)
	if err == nil {
		return t.Unix()
	}
	t, err = time.Parse("Mon Jan 02 15:04:05 -0700 2006", date)
	if err == nil {
		return t.Unix()
	}
	return -1
}
