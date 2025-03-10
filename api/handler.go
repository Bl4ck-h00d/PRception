package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"prception/core/github"
	"prception/core/llm"
	"strconv"
	"strings"
)

func HandleWebhook(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("Webhook triggered")

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		action, _ := payload["action"].(string)

		switch action {
		case "created":
			if payload["comment"] != nil {
				handleCommentEvent(token, payload)
			}
		case "opened":
			handlePROpenedEvent(token, payload)
		case "synchronize": // Handle new commits pushed to the PR
			handlePROpenedEvent(token, payload)
		default:
			http.Error(w, "Invalid action", http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// Handles comments mentioning @prception
func handleCommentEvent(token string, payload map[string]interface{}) {
	// Extract comment body
	commentBody := payload["comment"].(map[string]interface{})["body"].(string)

	if !strings.Contains(commentBody, "@prception") {
		return
	}

	owner := payload["repository"].(map[string]interface{})["owner"].(map[string]interface{})["login"].(string)
	repo := payload["repository"].(map[string]interface{})["name"].(string)
	prNumber := int(payload["issue"].(map[string]interface{})["number"].(float64))
	user := payload["comment"].(map[string]interface{})["user"].(map[string]interface{})["login"].(string)

	question := strings.TrimSpace(strings.Replace(commentBody, "@prception", "", 1))

	log.Printf("Processing question from @%s: %s", user, question)

	// Fetch PR diff and list of changed files for context
	diff, err := github.GetPRDiff(token, owner, repo, prNumber)
	if err != nil {
		log.Printf("Failed to get PR diff: %v", err)
		return
	}

	files, err := github.GetPRFiles(token, owner, repo, prNumber)
	if err != nil {
		log.Printf("Failed to get PR files: %v", err)
		return
	}

	diffMap := parseDiff(diff)

	// Prepare context for LLM analysis
	context := ""
	for _, path := range files {
		fileDiff, exists := diffMap[path]
		if !exists {
			continue
		}

		// Fetch full file content for better analysis
		fileContent, err := github.GetFileContent(token, owner, repo, path, "main")
		if err != nil {
			continue
		}

		// Append diff and file content to context
		context += fmt.Sprintf("### File: %s\n\n%s\n\n%s\n", path, fileDiff, fileContent)
	}

	// Send the context + question to LLM for analysis
	response, err := llm.ReviewCommentWithLLM(context + "\n\n" + question)
	if err != nil {
		log.Printf("Failed to get response from LLM: %v", err)
		return
	}

	// Post the response as a new comment
	comment := fmt.Sprintf("@%s %s", user, response)
	err = github.PostPRComment(token, owner, repo, prNumber, comment)
	if err != nil {
		log.Printf("Failed to post comment: %v", err)
		return
	}

	log.Printf("Replied to @%s: %s", user, response)
}

// Handles new PR open events
func handlePROpenedEvent(token string, payload map[string]interface{}) {
	// Extract repository and PR details
	owner := payload["repository"].(map[string]interface{})["owner"].(map[string]interface{})["login"].(string)
	repo := payload["repository"].(map[string]interface{})["name"].(string)
	prNumber := int(payload["pull_request"].(map[string]interface{})["number"].(float64))
	ref := payload["pull_request"].(map[string]interface{})["head"].(map[string]interface{})["ref"].(string)

	diff, err := github.GetPRDiff(token, owner, repo, prNumber)
	if err != nil {
		log.Printf("Failed to get PR diff: %v", err)
		return
	}

	files, err := github.GetPRFiles(token, owner, repo, prNumber)
	if err != nil {
		log.Printf("Failed to get PR files: %v", err)
		return
	}

	diffMap := parseDiff(diff)

	for _, path := range files {
		fileDiff, exists := diffMap[path]
		if !exists {
			continue
		}

		fileContent, err := github.GetFileContent(token, owner, repo, path, ref)
		if err != nil {
			continue
		}

		review, err := llm.ReviewCodeWithLLM(addLineNumbers(fileDiff), fileContent, path)
		if err != nil {
			continue
		}

		err = github.PostPRComment(token, owner, repo, prNumber, review)
		if err != nil {
			continue
		}

		if strings.Contains(review, "LGTM") {
			err = github.ApprovePR(token, owner, repo, prNumber)
			if err != nil {
				continue
			}
			log.Printf("PR approved!")
		} else {
			log.Printf("PR not approved. Feedback provided.")
		}
	}
}

// parseDiff parses a unified diff string and maps each file to its corresponding diff content.
//
// Parameters:
//   - diff: A string containing the unified diff output, typically from a git diff command.
//
// Returns:
//   - A map where the keys are file paths (as strings) and the values are the diff content for each file.
func parseDiff(diff string) map[string]string {
	diffMap := make(map[string]string)

	var currentFile string
	var currentDiff []string

	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			// Store previous file diff
			if currentFile != "" && len(currentDiff) > 0 {
				diffMap[currentFile] = strings.Join(currentDiff, "\n")
			}

			// Extract filename from diff header
			parts := strings.Fields(line)
			if len(parts) > 2 {
				// Normalize path by trimming "a/" or "b/" prefixes
				currentFile = strings.TrimPrefix(parts[2], "a/")
				currentFile = strings.TrimPrefix(currentFile, "b/")
			}
			currentDiff = []string{}
		} else if currentFile != "" {
			currentDiff = append(currentDiff, line)
		}
	}

	// Save last file diff
	if currentFile != "" && len(currentDiff) > 0 {
		diffMap[currentFile] = strings.Join(currentDiff, "\n")
	}

	return diffMap
}

func addLineNumbers(diff string) string {
	lines := strings.Split(diff, "\n")
	var result []string

	lineNumber := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			// Extract starting line number from the hunk header
			// Example: @@ -1,5 +1,6 @@
			parts := strings.Fields(line)
			if len(parts) > 1 {
				startLine := strings.Split(parts[2], ",")[0]
				startLine = strings.TrimPrefix(startLine, "+")
				num, err := strconv.Atoi(startLine)
				if err == nil {
					lineNumber = num
				}
			}
			result = append(result, line)
			continue
		}

		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
			result = append(result, fmt.Sprintf("%d: %s", lineNumber, line))
			lineNumber++
		} else {
			result = append(result, fmt.Sprintf("%d: %s", lineNumber, line))
			lineNumber++
		}
	}

	return strings.Join(result, "\n")
}
