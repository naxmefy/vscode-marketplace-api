package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/mux"
)

type publisher struct {
	PublisherId   string `json:"publisherId"`
	PublisherName string
	DisplayName   string
	Flags         string
}

type file struct {
	AssetType string
	Source    string
}

type version struct {
	Version          string
	Flags            string
	LastUpdated      string
	Files            []file
	AssetURI         string `json:"assetUri"`
	FallbackAssetURI string `json:"fallbackAssetUri"`
}

type statistic struct {
	StatisticName string
	Value         float32
}

type target struct {
	Target        string
	TargetVersion string
}

type vss struct {
	Publisher           publisher
	ExtensionID         string `json:"extensionId"`
	ExtensionName       string
	DisplayName         string
	Flags               string
	LastUpdated         string
	PublishedDate       string
	ReleaseDate         string
	ShortDescription    string
	Versions            []version
	Categories          []string
	Tags                []string
	Statistics          []statistic
	InstallationTargets []target
	DeploymentType      int
}

type item struct {
	Publisher       string
	Extension       string
	Version         string
	Link            string
	DownloadLink    string
	APIDownloadLink string
	Details         vss
}

var marketplaceURL = "https://marketplace.visualstudio.com/items?itemName={{.Publisher}}.{{.Extension}}"
var marketplaceDownloadURL = "https://{{.Publisher}}.gallery.vsassets.io/_apis/public" +
	"/gallery/publisher/{{.Publisher}}/extension/{{.Extension}}/{{.Version}}" +
	"/assetbyname/Microsoft.VisualStudio.Services.VSIXPackage"

func (i item) templateLink(templateString string) string {
	tmpl, err := template.New("item").Parse(templateString)
	if err != nil {
		panic(err)
	}

	var doc bytes.Buffer
	err = tmpl.Execute(&doc, i)

	if err != nil {
		panic(err)
	}
	return doc.String()
}

func (i item) GetLink() string {
	return i.templateLink(marketplaceURL)
}

func (i item) GetDownloadLink() string {
	return i.templateLink(marketplaceDownloadURL)
}

func (i item) GetDetails() vss {
	doc, err := goquery.NewDocument(i.GetLink())
	if err != nil {
		panic(err)
	}
	content := doc.Find(".vss-extension").First().Contents().Text()

	var details vss
	json.Unmarshal([]byte(content), &details)
	return details
}

func getItem(r *http.Request) item {
	vars := mux.Vars(r)

	i := item{
		Publisher: vars["publisher"],
		Extension: vars["extension"],
		Version:   vars["version"],
	}

	i.Details = i.GetDetails()
	i.Version = i.Details.Versions[0].Version

	i.DownloadLink = i.GetDownloadLink()
	i.Link = i.GetLink()

	scheme := getScheme(r)
	i.APIDownloadLink = scheme + "://" + r.Host + "/" + i.Publisher + "/" + i.Extension + "/" + i.Version + ".VSIX"

	fmt.Println("Extension:", i.Publisher, i.Extension, i.Version)
	fmt.Println("Link:", i.GetLink())
	return i
}

func printMarketExtension(w http.ResponseWriter, r *http.Request) {
	item := getItem(r)
	s, _ := json.Marshal(item)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%s", s)
}

func downloadMarketExtension(w http.ResponseWriter, r *http.Request) {
	item := getItem(r)
	url := item.GetDownloadLink()

	w.Header().Set("Content-Disposition", "attachment; filename="+item.Extension+"-"+item.Version+".VSIX")
	w.Header().Set("Content-Type", "application/zip")
	w.WriteHeader(http.StatusOK)

	response, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(w, "Error while downloading %s - %v", url, err)
		return
	}
	defer response.Body.Close()

	n, err := io.Copy(w, response.Body)
	if err != nil {
		fmt.Println("Error while downloading", url, "-", err)
		return
	}
	fmt.Println(item, n, "bytes downloaded.")
}

func getScheme(r *http.Request) string {
	scheme := r.URL.Scheme
	if !r.URL.IsAbs() {
		scheme = "http"
	}

	return scheme
}

func main() {
	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	r := mux.NewRouter()
	r.HandleFunc("/{publisher}/{extension}", printMarketExtension).Methods("GET")
	r.HandleFunc("/{publisher}/{extension}/{version:[0-9.]+}.VSIX", downloadMarketExtension).Methods("GET")
	log.Fatal(http.ListenAndServe(":"+port, r))
	log.Println("running on")
}
