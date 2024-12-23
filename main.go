package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
)

type RSS struct {
	Channel struct {
		Items []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Image       struct {
		URL string `xml:"url,attr"`
	} `xml:"thumbnail"`
}

var allowedDomains = []string{"deviantart.com", "www.deviantart.com", "images-wixmp-ed30a86b8c4ca887773594c2.wixmp.com"}

var bookmarks = struct {
	sync.Mutex
	items []gin.H
}{}

const bookmarksFile = "bookmarks.json"

func main() {
	if err := loadBookmarks(); err != nil {
		fmt.Println("Error loading bookmarks:", err)
		os.Exit(1)
	}

	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()
	router.Static("/static", "./static")
	router.LoadHTMLGlob("templates/*")

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})
	router.GET("/search", searchHandler)
	router.GET("/image", proxyImageHandler)
	router.GET("/about", func(c *gin.Context) {
		c.HTML(http.StatusOK, "about.html", nil)
	})

	router.GET("/bookmarks", bookmarksHandler)
	router.POST("/addBookmark", addBookmarkHandler)
	router.POST("/removeBookmark", removeBookmarkHandler)

	fmt.Println("Server started on http://localhost:3000")
	router.Run(":3000")
}

func loadBookmarks() error {
	data, err := os.ReadFile(bookmarksFile)
	if os.IsNotExist(err) {
		bookmarks.items = []gin.H{}
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) == 0 {
		bookmarks.items = []gin.H{}
		return nil
	}
	return json.Unmarshal(data, &bookmarks.items)
}

func saveBookmarks() error {
	bookmarks.Lock()
	defer bookmarks.Unlock()
	data, err := json.MarshalIndent(bookmarks.items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(bookmarksFile, data, 0644)
}

func bookmarksHandler(c *gin.Context) {
	bookmarks.Lock()
	defer bookmarks.Unlock()

	c.HTML(http.StatusOK, "bookmarks.html", gin.H{"Bookmarks": bookmarks.items})
}

func addBookmarkHandler(c *gin.Context) {
	var bookmark gin.H
	if err := c.ShouldBindJSON(&bookmark); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid bookmark data"})
		return
	}

	bookmarks.Lock()
	bookmarks.items = append(bookmarks.items, bookmark)
	bookmarks.Unlock()

	if err := saveBookmarks(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save bookmark"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Bookmark added successfully"})
}

func removeBookmarkHandler(c *gin.Context) {
	link := c.PostForm("Link")
	if link == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid bookmark data"})
		return
	}

	bookmarks.Lock()
	defer bookmarks.Unlock()

	var updatedBookmarks []gin.H
	for _, bookmark := range bookmarks.items {
		if bookmark["Link"] != link {
			updatedBookmarks = append(updatedBookmarks, bookmark)
		}
	}
	bookmarks.items = updatedBookmarks

	if err := saveBookmarks(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save bookmarks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Bookmark removed successfully"})
}

func searchHandler(c *gin.Context) {
	URL := os.Getenv("URL")
	if URL == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server misconfiguration"})
		return
	}

	query := c.Query("q")
	if query == "" {
		c.HTML(http.StatusBadRequest, "results.html", gin.H{
			"Error": "No search query provided.",
		})
		return
	}

	rssFeedURL := fmt.Sprintf("https://backend.deviantart.com/rss.xml?q=%s", url.QueryEscape(query))
	resp, err := http.Get(rssFeedURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		c.HTML(http.StatusInternalServerError, "results.html", gin.H{
			"Error": "Failed to fetch RSS feed.",
		})
		return
	}
	defer resp.Body.Close()

	var rss RSS
	if err := xml.NewDecoder(resp.Body).Decode(&rss); err != nil {
		c.HTML(http.StatusInternalServerError, "results.html", gin.H{
			"Error": "Failed to parse RSS feed.",
		})
		return
	}

	var results []gin.H
	for _, item := range rss.Channel.Items {
		results = append(results, gin.H{
			"Title":       item.Title,
			"Link":        item.Link,
			"Image":       fmt.Sprintf("%s/image?url=%s", URL, url.QueryEscape(item.Image.URL)),
			"Description": sanitizeDescription(item.Description),
		})

	}

	c.HTML(http.StatusOK, "results.html", gin.H{
		"Results": results,
		"Query":   query,
	})
}

func sanitizeDescription(input string) string {
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return input
	}
	var b strings.Builder
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		if n.Type == html.ElementNode && n.Data == "br" {
			b.WriteString("\n")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(doc)
	return strings.TrimSpace(b.String())
}

func proxyImageHandler(c *gin.Context) {
	imageUrl := c.Query("url")

	imageSrc, err := fetchImage(imageUrl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch image"})
		return
	}

	c.Header("Content-Type", "image/png")
	c.Data(http.StatusOK, "image/png", imageSrc)
}

func fetchImage(imageUrl string) ([]byte, error) {
	resp, err := http.Get(imageUrl)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch image")
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
