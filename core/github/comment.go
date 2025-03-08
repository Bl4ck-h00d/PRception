package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

func PostPRComment(token, owner, repo string, prNumber int, comment string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments", owner, repo, prNumber)

	body, _ := json.Marshal(map[string]string{
		"body": comment,
	})

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to post comment: %s", resp.Status)
	}

	return nil
}
