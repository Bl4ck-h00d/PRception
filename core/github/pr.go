package github

import (
	"bytes"
	"fmt"
	"net/http"
)

func ApprovePR(token, owner, repo string, prNumber int) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/reviews", owner, repo, prNumber)

	body := []byte(`{"event": "APPROVE"}`)
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
		return fmt.Errorf("failed to approve PR: %s", resp.Status)
	}

	return nil
}
