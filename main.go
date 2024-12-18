package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

// ReadmeResponse Structure to match GitHub's README response
type ReadmeResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

func getReadmeContent(owner, repo string) (string, error) {
	ghToken := os.Getenv("GITHUB_TOKEN")

	// Construct the API URL
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/readme", owner, repo)

	// Create the request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	// Add necessary headers
	req.Header.Add("Accept", "application/vnd.github+json")
	// If you have a token, add it here
	req.Header.Add("Authorization", "Bearer "+ghToken)
	fmt.Printf("Authorization Header: %s\n", req.Header.Get("Authorization"))

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	// Check if the status code is successful
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse the JSON response
	var readmeResp ReadmeResponse
	if err := json.Unmarshal(body, &readmeResp); err != nil {
		return "", fmt.Errorf("error parsing JSON: %v", err)
	}

	// Remove newlines from content before decoding
	cleanContent := strings.Replace(readmeResp.Content, "\n", "", -1)

	// Decode the base64 content
	decodedBytes, err := base64.StdEncoding.DecodeString(cleanContent)
	if err != nil {
		return "", fmt.Errorf("error decoding base64: %v", err)
	}

	return string(decodedBytes), nil
}

func main() {

	owner := "proSamik"
	repo := "proSamik"

	content, err := getReadmeContent(owner, repo)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("README Content:")
	fmt.Println(content)

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Enter text (type 'exit' to quit):")

	for scanner.Scan() {
		text := scanner.Text()
		if strings.ToLower(text) == "exit" {
			break
		}
		fmt.Printf("You entered: %s\n", text)
	}
}
