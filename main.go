package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/parser"
	"golang.org/x/net/html"
)

// MarkdownDocument Comprehensive Markdown Element Structures
type MarkdownDocument struct {
	Metadata   DocumentMetadata `json:"metadata"`
	Content    []Element        `json:"content"`
	RawContent string           `json:"rawContent"`
}

type DocumentMetadata struct {
	Title       string    `json:"title"`
	Repository  string    `json:"repository"`
	LastUpdated time.Time `json:"lastUpdated"`
	Author      string    `json:"author"`
	Description string    `json:"description"`
}

type Element struct {
	Type       string     `json:"type"`
	Content    string     `json:"content,omitempty"`
	Children   []Element  `json:"children,omitempty"`
	Attributes Attributes `json:"attributes,omitempty"`
}

type Attributes struct {
	Href   string `json:"href,omitempty"`
	Src    string `json:"src,omitempty"`
	Alt    string `json:"alt,omitempty"`
	Title  string `json:"title,omitempty"`
	Width  string `json:"width,omitempty"`
	Height string `json:"height,omitempty"`
	Level  string `json:"level,omitempty"`
}

// Markdown Parsing Function
func parseMarkdownToHTML(markdownContent []byte) string {
	// Configure Markdown parser
	extensions := parser.CommonExtensions |
		parser.AutoHeadingIDs |
		parser.HardLineBreak |
		parser.NoEmptyLineBeforeBlock

	mdParser := parser.NewWithExtensions(extensions)

	// Convert markdown to HTML
	htmlContent := markdown.ToHTML(markdownContent, mdParser, nil)

	return string(htmlContent)
}

// HTML Parsing Function
func parseHTMLToElements(htmlContent string) []Element {
	// Create a new HTML tokenizer
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		log.Printf("Error parsing HTML: %v", err)
		return []Element{}
	}

	var elements []Element

	// Recursive function to traverse HTML nodes
	var traverse func(*html.Node) []Element
	traverse = func(n *html.Node) []Element {
		if n == nil {
			return []Element{}
		}

		var nodeElements []Element

		// Process different node types
		switch nodeType := n.Type; nodeType {
		case html.ElementNode:
			switch n.Data {
			case "h1", "h2", "h3", "h4", "h5", "h6":
				// Heading
				level := strings.TrimPrefix(n.Data, "h")
				element := Element{
					Type:    "heading",
					Content: extractNodeText(n),
					Attributes: Attributes{
						Level: level,
					},
				}
				nodeElements = append(nodeElements, element)

			case "p":
				// Paragraph
				para := Element{
					Type:     "paragraph",
					Children: traverse(n.FirstChild),
				}
				nodeElements = append(nodeElements, para)

			case "a":
				// Link
				href := getAttr(n, "href")
				link := Element{
					Type: "link",
					Attributes: Attributes{
						Href: href,
					},
					Children: traverse(n.FirstChild),
				}
				nodeElements = append(nodeElements, link)

			case "img":
				// Image
				img := Element{
					Type: "image",
					Attributes: Attributes{
						Src: getAttr(n, "src"),
						Alt: getAttr(n, "alt"),
					},
				}
				nodeElements = append(nodeElements, img)

			case "code":
				// Inline code
				code := Element{
					Type:    "code",
					Content: extractNodeText(n),
				}
				nodeElements = append(nodeElements, code)

			case "pre":
				// Code block
				codeBlock := Element{
					Type:    "code_block",
					Content: extractNodeText(n),
				}
				nodeElements = append(nodeElements, codeBlock)

			case "strong", "b":
				// Bold text
				strong := Element{
					Type:     "strong",
					Children: traverse(n.FirstChild),
				}
				nodeElements = append(nodeElements, strong)

			case "em", "i":
				// Italic text
				em := Element{
					Type:     "emphasis",
					Children: traverse(n.FirstChild),
				}
				nodeElements = append(nodeElements, em)

			case "ul":
				// Unordered list
				list := Element{
					Type:     "unordered_list",
					Children: traverse(n.FirstChild),
				}
				nodeElements = append(nodeElements, list)

			case "ol":
				// Ordered list
				list := Element{
					Type:     "ordered_list",
					Children: traverse(n.FirstChild),
				}
				nodeElements = append(nodeElements, list)

			case "li":
				// List item
				listItem := Element{
					Type:     "list_item",
					Children: traverse(n.FirstChild),
				}
				nodeElements = append(nodeElements, listItem)

			case "table":
				// Table
				table := Element{
					Type:     "table",
					Children: traverse(n.FirstChild),
				}
				nodeElements = append(nodeElements, table)

			case "tr":
				// Table row
				row := Element{
					Type:     "table_row",
					Children: traverse(n.FirstChild),
				}
				nodeElements = append(nodeElements, row)

			case "th":
				// Table header cell
				headerCell := Element{
					Type:     "table_header_cell",
					Content:  extractNodeText(n),
					Children: traverse(n.FirstChild),
				}
				nodeElements = append(nodeElements, headerCell)

			case "td":
				// Table cell
				cell := Element{
					Type:     "table_cell",
					Content:  extractNodeText(n),
					Children: traverse(n.FirstChild),
				}
				nodeElements = append(nodeElements, cell)

			}

		case html.TextNode:
			// Plain text
			if strings.TrimSpace(n.Data) != "" {
				text := Element{
					Type:    "text",
					Content: strings.TrimSpace(n.Data),
				}
				nodeElements = append(nodeElements, text)
			}

		default:
			// Handle any unmatched element types
			log.Printf("Unhandled element type: %s", n.Data)
		}

		// Traverse siblings
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			nodeElements = append(nodeElements, traverse(c)...)
		}

		return nodeElements
	}

	// Start traversing from the root
	elements = traverse(doc)

	return elements
}

// Helper function to extract text from HTML node
func extractNodeText(n *html.Node) string {
	var text string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			text += c.Data
		}
	}
	return strings.TrimSpace(text)
}

// Helper function to get attribute value
func getAttr(n *html.Node, attr string) string {
	for _, a := range n.Attr {
		if a.Key == attr {
			return a.Val
		}
	}
	return ""
}

// Updated GitHub API interaction functions with improved error handling
func getReadmeContent(ctx context.Context, owner, repo string) (string, error) {
	token := os.Getenv("GITHUB_TOKEN")
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/readme", owner, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("making request: %w", err)
	}

	// Improved response body closure with error handling
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			log.Printf("Error closing response body: %v", closeErr)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	var readmeResp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(body, &readmeResp); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	decodedContent, err := base64.StdEncoding.DecodeString(
		strings.ReplaceAll(readmeResp.Content, "\n", ""),
	)
	if err != nil {
		return "", fmt.Errorf("decoding content: %w", err)
	}

	return string(decodedContent), nil
}

func getRepositoryMetadata(ctx context.Context, owner, repo string) (DocumentMetadata, error) {
	token := os.Getenv("GITHUB_TOKEN")
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return DocumentMetadata{}, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return DocumentMetadata{}, fmt.Errorf("making request: %w", err)
	}
	// Improved response body closure with error handling
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			log.Printf("Error closing response body: %v", closeErr)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return DocumentMetadata{}, fmt.Errorf("reading response: %w", err)
	}

	var repoResp struct {
		Name        string    `json:"name"`
		Description string    `json:"description"`
		UpdatedAt   time.Time `json:"updated_at"`
		Owner       struct {
			Login string `json:"login"`
		} `json:"owner"`
	}
	if err := json.Unmarshal(body, &repoResp); err != nil {
		return DocumentMetadata{}, fmt.Errorf("parsing response: %w", err)
	}

	// Extract first line from README as title
	loc, _ := time.LoadLocation("Asia/Kolkata")

	return DocumentMetadata{
		Title:       extractFirstLineFromReadme(repoResp.Name, repoResp.Description),
		Repository:  fmt.Sprintf("%s/%s", owner, repo),
		LastUpdated: repoResp.UpdatedAt.In(loc),
		Author:      repoResp.Owner.Login,
		Description: repoResp.Description,
	}, nil
}

// Helper function to extract first meaningful line
func extractFirstLineFromReadme(repoName, description string) string {
	// Prioritize description if available
	if description != "" {
		return description
	}

	// Fallback to repository name
	return repoName
}

// HTTP Handler for README Processing
func handleReadmeRequest(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Content-Type", "application/json")

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Extract query parameters
	owner := r.URL.Query().Get("owner")
	repo := r.URL.Query().Get("repo")

	if owner == "" || repo == "" {
		http.Error(w, "Owner and repository are required", http.StatusBadRequest)
		return
	}

	// Process README
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	doc, err := processReadme(ctx, owner, repo)
	if err != nil {
		log.Printf("Error processing README: %v", err)
		http.Error(w, "Failed to process README", http.StatusInternalServerError)
		return
	}

	// Encode and send response
	if err := json.NewEncoder(w).Encode(doc); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// Process README
func processReadme(ctx context.Context, owner, repo string) (MarkdownDocument, error) {
	// Fetch README content
	readmeContent, err := getReadmeContent(ctx, owner, repo)
	if err != nil {
		return MarkdownDocument{}, fmt.Errorf("fetching readme: %w", err)
	}

	// Convert Markdown to HTML
	htmlContent := parseMarkdownToHTML([]byte(readmeContent))

	// Parse HTML to structured elements
	parsedContent := parseHTMLToElements(htmlContent)

	// Get repository metadata
	metadata, err := getRepositoryMetadata(ctx, owner, repo)
	if err != nil {
		return MarkdownDocument{}, fmt.Errorf("fetching metadata: %w", err)
	}

	return MarkdownDocument{
		Metadata:   metadata,
		Content:    parsedContent,
		RawContent: readmeContent,
	}, nil
}

func main() {
	// Validate GitHub Token
	if os.Getenv("GITHUB_TOKEN") == "" {
		log.Fatal("GITHUB_TOKEN environment variable is not set")
	}

	// Configure routes
	http.HandleFunc("/readme", handleReadmeRequest)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
