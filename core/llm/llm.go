package llm

import (
	"context"
	"fmt"
	"os"

	openai "github.com/sashabaranov/go-openai"
)

var client *openai.Client

func init() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	client = openai.NewClient(apiKey)
}

func ReviewCodeWithLLM(diff string, fileContent string) (string, error) {
	ctx := context.Background()

	prompt := fmt.Sprintf(`You are an experienced code reviewer. Your task is to review the following code changes carefully:

### Original Code:
%s

### Changes:
%s

### Instructions:
1. Identify any bugs, vulnerabilities, or logical errors.
2. Suggest improvements for code readability, performance, and maintainability.
3. If there are any best practices violated, mention them.
4. If the changes are good and meet coding standards, provide approval.

### Response Format:
- **Issues:** (List issues and their impact)
- **Suggestions:** (List improvements with reasoning)
- **Approval Status:** ("LGTM" or "Needs changes")`, fileContent, diff)

	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    "system",
				Content: "You are a professional code reviewer. Provide structured feedback on the changes.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens: 1000,
	})
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return resp.Choices[0].Message.Content, nil
}
