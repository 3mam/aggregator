package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/mmcdole/gofeed"
)

type RSS struct {
	Name string  `xml:"title"`
	Item []entry `xml:"entry"`
	Atom struct {
		Name string `xml:"title"`
		Item []item `xml:"item"`
	} `xml:"channel"`
	Atom2 struct {
		Name string `xml:"title"`
		Item []item `xml:"item"`
	} `xml:"rss>channel"`
}

type entry struct {
	Title string `xml:"title"`
	Date  string `xml:"published"`
	Link  struct {
		Href string `xml:"href,attr"`
	} `xml:"link"`
}

type item struct {
	Title  string `xml:"title"`
	Link   string `xml:"link"`
	Guid   string `xml:"guid"`
	Date   string `xml:"pubDate"`
	Id     int64
	Parent int64
}

type OPML struct {
	_       byte      `xml:"opml"`
	Outline []outline `xml:"body>outline"`
}

type outline struct {
	Title   string    `xml:"title,attr"`
	Type    string    `xml:"type,attr"`
	XmlUrl  string    `xml:"xmlUrl,attr"`
	Outline []outline `xml:"outline"`
	Node    int64
	Parent  int64
	Color   string
	Items   []*gofeed.Item
}

type rssItem struct {
	Parent int64
	Title  string
	Link   string
	Date   int64
}

func rss(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.RawQuery == "":
		rssRoot(w, r)
	case r.URL.RawQuery == "manager":
		rssManager(w, r)
	case strings.Contains(r.URL.RawQuery, "read"):
		if r.URL.RawQuery[:len("read")] == "read" {
			rssRead(w, r)
		}
	case strings.Contains(r.URL.RawQuery, "edit"):
		if r.URL.RawQuery[:len("edit")] == "edit" {
			rssEdit(w, r)
		}
	case strings.Contains(r.URL.RawQuery, "delete"):
		if r.URL.RawQuery[:len("delete")] == "delete" {
			rssDelete(w, r)
		}
	case strings.Contains(r.URL.RawQuery, "add"):
		if r.URL.RawQuery[:len("add")] == "add" {
			rssAdd(w, r)
		}
	}
}

func rssAdd(w http.ResponseWriter, r *http.Request) {
	var db, _ = sql.Open("sqlite3", "./agregator.db")
	defer db.Close()
	if r.Method == "POST" {
		var xmlurl string
		xmlurl = r.FormValue("xmlurl")
		var client = &http.Client{}
		var req, errReq = http.NewRequest("GET", xmlurl, nil)
		if errReq != nil {
			log.Fatalln(errReq)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:51.0) Gecko/20100101 Firefox/51.0")

		var resp, errorResp = client.Do(req)
		if errorResp != nil {
			fmt.Printf("%s: %v\n", xmlurl, errorResp)
			return
		}
		var fp = gofeed.NewParser()
		var rssTmp, err = fp.Parse(resp.Body)
		if err != nil {
			fmt.Println(rssTmp, err)
			return
		}
		resp.Body.Close()
		var db, _ = sql.Open("sqlite3", "./agregator.db")
		var tx, _ = db.Begin()
		var stmt, _ = tx.Prepare("insert into rss(node, parent, title, type, xmlurl) values(?, ?, ?, ?, ?)")
		stmt.Exec(rand.Int63(), r.FormValue("type"), rssTmp.Title, "atom", xmlurl)
		stmt.Close()
		tx.Commit()
		http.Redirect(w, r, "/rss?manager", http.StatusFound)
		return
	}
	var options string
	var node string
	var title string
	var rows, _ = db.Query("select node, title from rss where type='folder'")
	for rows.Next() {
		rows.Scan(&node, &title)
		options += "<option value=\"" + node + "\">" + title + "</option>"
	}
	rows.Close()
	type editHtml struct {
		Type string
	}
	var buf = new(bytes.Buffer)
	var t, _ = template.New("rssAdd.html").ParseFiles("html/rssAdd.html")
	t.Execute(buf, &editHtml{options})
	w.Write(buf.Bytes())
}

func rssDelete(w http.ResponseWriter, r *http.Request) {
	var db, _ = sql.Open("sqlite3", "./agregator.db")
	defer db.Close()
	if r.Method == "POST" {
		db.Exec("delete from rss where node=?", r.URL.RawQuery[len("delete="):])
		http.Redirect(w, r, "/rss?manager", http.StatusFound)
		return
	}
	var buf = new(bytes.Buffer)
	var t, _ = template.New("rssDelete.html").ParseFiles("html/rssDelete.html")
	type editHtml struct {
		Node   string
		Title  string
	}
	var title string
	var stmt, _ = db.Prepare("select title from rss where node=?")
	stmt.QueryRow(r.URL.RawQuery[len("delete="):]).Scan(&title)

	t.Execute(buf, &editHtml{r.URL.RawQuery[len("delete="):], title})
	w.Write(buf.Bytes())

}

func rssEdit(w http.ResponseWriter, r *http.Request) {
	var node int64
	var parent int64
	var title string
	var folderName string
	var types string
	var xmlurl string
	var options string
	var db, _ = sql.Open("sqlite3", "./agregator.db")
	defer db.Close()
	if r.Method == "POST" {
		var stmt, _ = db.Prepare("update rss set title=?, parent=?, xmlurl=? where node=?")
		//parent, _ = strconv.ParseInt(r.FormValue("type"), 10, 64)
		stmt.Exec(r.FormValue("title"), r.FormValue("type"), r.FormValue("xmlurl"), r.URL.RawQuery[len("edit="):])
		stmt.Close()
		http.Redirect(w, r, "/rss?manager", http.StatusFound)
		return
	}

	var stmt, _ = db.Prepare("select parent, title, type, xmlurl from rss where node=?")
	stmt.QueryRow(r.URL.RawQuery[len("edit="):]).Scan(&parent, &title, &types, &xmlurl)

	var buf = new(bytes.Buffer)
	var t, _ = template.New("rssEdit.html").ParseFiles("html/rssEdit.html")

	type editHtml struct {
		Node   string
		Title  string
		Xmlurl string
		Type   string
	}
	stmt, _ = db.Prepare("select title from rss where node=?")
	stmt.QueryRow(parent).Scan(&types)
	options += "<option value=\"" + strconv.FormatInt(parent, 10) + "\">" + types + "</option>"

	var rows, _ = db.Query("select node, title, type from rss")
	for rows.Next() {
		rows.Scan(&node, &folderName, &types)
		if types == "folder" && parent != node {
			options += "<option value=\"" + strconv.FormatInt(node, 10) + "\">" + folderName + "</option>"
		}
	}
	rows.Close()
	t.Execute(buf, &editHtml{r.URL.RawQuery[len("edit="):], title, xmlurl, options})
	w.Write(buf.Bytes())

}

func rssRead(w http.ResponseWriter, r *http.Request) {
	defer http.Redirect(w, r, "/rss", http.StatusFound)
	var list []int64
	var db, _ = sql.Open("sqlite3", "./agregator.db")
	var tx, _ = db.Begin()
	if json.Unmarshal([]byte(r.URL.RawQuery[len("read="):]), &list) != nil {
		return
	}
	var stmt, _ = tx.Prepare("update items set read=1 where id=?")
	for i := range list {
		stmt.Exec(list[i])
	}
	stmt.Close()
	tx.Commit()
	db.Close()
}

func table(name, color, title, link string) (url string) {
	url += "<tr>"
	//	url += "<td>" + time.Now().Format("2/1/2006 15:04") + "</td>"
	url += "<td bgcolor=\"" + color + "\"" + ">" + name + "</td>\n"
	url += "<td><a class=\"button\" target=\"_blank\" href=\"" + link + "\">" + title + "</a></td>\n"
	url += "</tr>"
	return
}

func rssRoot(w http.ResponseWriter, r *http.Request) {
	var html string
	var list []int64
	var db, _ = sql.Open("sqlite3", "./agregator.db")
	var rows, _ = db.Query("select id, parent, title, url from items where read=0")
	var o item
	html += "<p>"
	for n := range gNodeList {
		if gNodeList[n].Color != "" {
			html += "<font color=\"" + gNodeList[n].Color + "\">" + gNodeList[n].Title + " </font> "
		}
	}
	html += "</p>"
	for rows.Next() {
		rows.Scan(&o.Id, &o.Parent, &o.Title, &o.Link)
		if gNodeList[o.Parent].Color == "" {

			html += table(gNodeList[o.Parent].Title, gNodeList[gNodeList[o.Parent].Parent].Color, o.Title, o.Link)
		}
		list = append(list, o.Id)
	}
	rows.Close()
	db.Close()
	var jsonList, _ = json.Marshal(list)
	var buf = new(bytes.Buffer)
	var t, _ = template.New("rss.html").ParseFiles("html/rss.html")
	t.Execute(buf, &Page{"agregator", html, string(jsonList)})
	w.Write(buf.Bytes())
}

func treeFromOpmlToSql(outline []outline, parent int64, stmt *sql.Stmt) {
	for i, o := range outline {
		outline[i].Node = rand.Int63()
		if o.Outline != nil {
			stmt.Exec(outline[i].Node, parent, outline[i].Title, outline[i].Type, outline[i].XmlUrl)
			treeFromOpmlToSql(o.Outline, outline[i].Node, stmt)
		} else {
			stmt.Exec(outline[i].Node, parent, outline[i].Title, outline[i].Type, outline[i].XmlUrl)
		}
	}
}

func sortDeepTreeFromSql(in outline, out []outline) {
	for i := range out {
		if out[i].Node == in.Parent {
			out[i].Outline = append(out[i].Outline, in)
		} else if out[i].Outline != nil {
			sortDeepTreeFromSql(in, out[i].Outline)
		}
	}
}

func sortTreeFromSql(in []outline) (sort []outline) {
	for i := range in {
		if in[i].Parent == 0 {
			sort = append(sort, in[i])
		}
	}
	for i := range in {
		sortDeepTreeFromSql(in[i], sort)
	}
	return
}

func treeToHtml(outline []outline) (treeHtmlList string) {
	for _, o := range outline {
		treeHtmlList += "<ul><li>" + "<label class=\"button\"> <div class=\"tooltip\">" + o.Title + 
		"<span class=\"tooltiptext\">"+
		"<a class=\"url\" href=\"/rss?edit=" + strconv.FormatInt(o.Node, 10) + "\">edytuj</a> " + 
		"<a class=\"url\" href=\"/rss?delete=" + strconv.FormatInt(o.Node, 10) + "\">usuń</a> " + 
		"</span></div></label>"
		if o.Outline != nil {
			treeHtmlList += treeToHtml(o.Outline)
		}
		treeHtmlList += "</ul></li>"
	}
	return
}

func rssManager(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var o OPML
		var file, _, _ = r.FormFile("files")
		var f, _ = ioutil.ReadAll(file)
		var errorXml = xml.Unmarshal(f, &o)
		if errorXml != nil {
			fmt.Fprintf(w, "error: %v", errorXml)
			return
		}
		var db, _ = sql.Open("sqlite3", "./agregator.db")
		var tx, _ = db.Begin()
		var stmt, _ = tx.Prepare("insert into rss(node, parent, title, type, xmlurl) values(?, ?, ?, ?, ?)")
		treeFromOpmlToSql(o.Outline, 0, stmt)
		stmt.Close()
		tx.Commit()
		db.Close()
	}
	var rssList = loadRssFromSql()
	var buf = new(bytes.Buffer)
	var t, _ = template.New("rssManager.html").ParseFiles("html/rssManager.html")
	t.Execute(buf, &Page{"", treeToHtml(sortTreeFromSql(rssList)), ""})
	w.Write(buf.Bytes())
}

func loadRssFromSql() (rssList []outline) {
	var db, _ = sql.Open("sqlite3", "./agregator.db")
	var rows, _ = db.Query("select * from rss")
	for rows.Next() {
		var t outline
		rows.Scan(&t.Node, &t.Parent, &t.Title, &t.Type, &t.XmlUrl, &t.Color)
		gNodeList[t.Node] = NodeList{t.Parent, t.Title, t.Color}
		rssList = append(rssList, t)
	}
	rows.Close()
	db.Close()
	return
}

func rssUpdate(db *sql.DB) {
	var rssItemList = loadRssFromSql()
	var rssList []rssItem
	for i, r := range rssItemList {
		if r.Type == "folder" {
			continue
		}
		//	if strings.Contains(r.XmlUrl, "youtube") {
		//		continue
		//	}
		var client = &http.Client{}
		var req, errReq = http.NewRequest("GET", r.XmlUrl, nil)
		if errReq != nil {
			log.Fatalln(errReq)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:51.0) Gecko/20100101 Firefox/51.0")

		var resp, errorResp = client.Do(req)
		if errorResp != nil {
			fmt.Printf("%s: %v\n", r.Title, errorResp)
			continue
		}
		var fp = gofeed.NewParser()
		var rssTmp, err = fp.Parse(resp.Body)
		if err != nil {
			fmt.Println(err)
			continue
		}
		rssItemList[i].Items = rssTmp.Items
		resp.Body.Close()

	}
	var tx, _ = db.Begin()
	var stmt, _ = tx.Prepare("select url from items where url = ?")
	for _, v := range rssItemList {
		for _, s := range v.Items {
			var url string
			stmt.QueryRow(s.Link).Scan(&url)
			if url != s.Link && s.PublishedParsed.Unix() >= time.Now().AddDate(0, 0, TIME_DELETE).Unix() {
				rssList = append(rssList, rssItem{v.Node, s.Title, s.Link, s.PublishedParsed.Unix()})
			}
		}
	}
	stmt.Close()
	tx.Commit()
	tx, _ = db.Begin()
	stmt, _ = tx.Prepare("insert or ignore into items(id, parent, title, url, time, read) values(?,?,?,?,?,0)")
	for _, s := range rssList {
		stmt.Exec(rand.Int63(), s.Parent, s.Title, s.Link, s.Date)
	}

	stmt.Close()
	tx.Commit()
}
