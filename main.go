package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/antchfx/xpath"
)

var (
	wg               = sync.WaitGroup{}
	chn              = make(chan WHOIS, 64) //控制抓取线程数
	availableCounter = 0
	host             = "https://www.iana.org"

	registryExpr = xpath.MustCompile("//*[@id=\"main_right\"]/h2[5]")
	textExpr     = xpath.MustCompile("//*[@id=\"main_right\"]/p[2]/text()")
)

// Domain 根域
type Domain struct {
	Domain     string
	Details    string
	Type       string
	TLDManager string
}

// WHOIS WHOIS服务器
type WHOIS struct {
	Domain string
	WHOIS  string
}

func main() {
	/*
		1. 抓取根域列表
		2. 抓取某个根域的WHOIS服务器
		3. 持久化WHOIS数据
	*/

	rootdb := fetchRootDB()

	go domainToTxt()

	for _, dom := range rootdb {
		wg.Add(1)
		go fetchWHOIS(dom)
	}

	wg.Wait()
}

func fetchWHOIS(dom Domain) {
	defer wg.Done()

	doc, err := htmlquery.LoadURL(host + dom.Details)
	checkErr(err)

	servNode, err := htmlquery.Query(doc, "//*[@id=\"main_right\"]/p[2]/b[2]")
	checkErr(err)

	if servNode == nil {
		return
	}

	data := strings.TrimSpace(servNode.NextSibling.Data)

	chn <- WHOIS{
		Domain: dom.Domain,
		WHOIS:  data,
	}
}

func domainToTxt() {
	//WHOIS信息持久化保存

	txtName := fmt.Sprintf("WHOIS_%v.txt", time.Now().Format("20060102150405"))

	f, err := os.OpenFile(txtName, os.O_CREATE|os.O_RDWR|os.O_APPEND, os.ModePerm)
	checkErr(err)

	for {
		w := <-chn
		availableCounter++
		f.WriteString(fmt.Sprintf("%v: %v\n", w.Domain, w.WHOIS))

		fmt.Printf("|%06d| %v | %v\n", availableCounter, w.Domain, w.WHOIS)
	}
}

func fetchRootDB() (rootdb []Domain) {
	doc, err := htmlquery.LoadURL("https://www.iana.org/domains/root/db")
	checkErr(err)

	rows, err := htmlquery.QueryAll(doc, "//*[@id=\"tld-table\"]/tbody/tr")
	checkErr(err)

	domainExpr := xpath.MustCompile("//td[1]/span/a")
	typeExpr := xpath.MustCompile("//td[2]")
	managerExpr := xpath.MustCompile("//td[3]")

	for _, row := range rows {
		domain := htmlquery.QuerySelector(row, domainExpr)
		domtype := htmlquery.QuerySelector(row, typeExpr)
		manager := htmlquery.QuerySelector(row, managerExpr)

		dom := Domain{
			Domain:     domain.FirstChild.Data,
			Details:    domain.Attr[0].Val,
			Type:       domtype.FirstChild.Data,
			TLDManager: manager.FirstChild.Data,
		}

		rootdb = append(rootdb, dom)
	}

	return
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
