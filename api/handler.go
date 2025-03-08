package api

import (
	"encoding/json"
	"log"
	"net/http"
	"prception/core/github"
	"prception/core/llm"
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

		action, ok := payload["action"].(string)
		if !ok || action != "opened" {
			http.Error(w, "Invalid action", http.StatusBadRequest)
			return
		}

		owner := payload["repository"].(map[string]interface{})["owner"].(map[string]interface{})["login"].(string)
		repo := payload["repository"].(map[string]interface{})["name"].(string)
		prNumber := int(payload["pull_request"].(map[string]interface{})["number"].(float64))
		ref := payload["pull_request"].(map[string]interface{})["head"].(map[string]interface{})["ref"].(string)

		// Get PR diff
		diff, err := github.GetPRDiff(token, owner, repo, prNumber)

		if err != nil {
			log.Printf("Failed to get PR diff: %v", err)
			http.Error(w, "Failed to get PR diff", http.StatusInternalServerError)
			return
		}

		//  Get list of changed files
		files, err := github.GetPRFiles(token, owner, repo, prNumber)
		if err != nil {
			log.Printf("Failed to get PR files: %v", err)
			http.Error(w, "Failed to get PR files", http.StatusInternalServerError)
			return
		}

		// map diff with files
		diffMap := parseDiff(diff)

		for _, path := range files {
			fileDiff, exists := diffMap[path]
			if !exists {
				log.Printf("No diff found for file: %s", path)
				continue
			}

			// Get file content for matching file
			fileContent, err := github.GetFileContent(token, owner, repo, path, ref)
			if err != nil {
				log.Printf("Failed to get file content for %s: %v", path, err)
				continue
			}

			// Review only the relevant diff and content
			review, err := llm.ReviewCodeWithLLM(fileDiff, fileContent)
			if err != nil {
				log.Printf("Failed to review code with LLM: %v", err)
				continue
			}

			// Post feedback as a comment
			err = github.PostPRComment(token, owner, repo, prNumber, review)
			if err != nil {
				log.Printf("Failed to post comment: %v", err)
				continue
			}

			// Approve PR if feedback is positive
			if strings.Contains(review, "LGTM") {
				err = github.ApprovePR(token, owner, repo, prNumber)
				if err != nil {
					log.Printf("Failed to approve PR: %v", err)
					continue
				}
				log.Printf("PR approved!")
			} else {
				log.Printf("PR not approved. Feedback provided.")

			}
		}

		w.WriteHeader(http.StatusOK)
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
