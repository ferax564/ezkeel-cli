package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Interact with AI agents on the platform",
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available agents",
	RunE:  runAgentList,
}

var agentRunCmd = &cobra.Command{
	Use:   "run <agent> <prompt...>",
	Short: "Run a single prompt against an agent",
	Args:  cobra.MinimumNArgs(2),
	RunE:  runAgentRun,
}

var agentChatCmd = &cobra.Command{
	Use:   "chat <agent>",
	Short: "Start an interactive chat session with an agent",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentChat,
}

func init() {
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentRunCmd)
	agentCmd.AddCommand(agentChatCmd)

	agentCmd.PersistentFlags().String("url", "", "Platform URL (e.g. https://app.ezkeel.com)")
	agentCmd.PersistentFlags().String("token", "", "JWT auth token")
}

// agentPlatformURL resolves the platform URL from flag or EZKEEL_PLATFORM_URL env var.
func agentPlatformURL(cmd *cobra.Command) string {
	u, _ := cmd.Flags().GetString("url")
	if u != "" {
		return strings.TrimRight(u, "/")
	}
	if env := os.Getenv("EZKEEL_PLATFORM_URL"); env != "" {
		return strings.TrimRight(env, "/")
	}
	return "https://app.ezkeel.com"
}

// agentAuthToken resolves the auth token from flag or EZKEEL_TOKEN env var.
func agentAuthToken(cmd *cobra.Command) (string, error) {
	t, _ := cmd.Flags().GetString("token")
	if t != "" {
		return t, nil
	}
	if env := os.Getenv("EZKEEL_TOKEN"); env != "" {
		return env, nil
	}
	return "", fmt.Errorf("auth token required: use --token flag or EZKEEL_TOKEN env var")
}

func runAgentList(cmd *cobra.Command, _ []string) error {
	baseURL := agentPlatformURL(cmd)
	token, err := agentAuthToken(cmd)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", baseURL+"/api/agents", nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Cookie", "session="+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("contacting platform: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("platform returned %d: %s", resp.StatusCode, string(body))
	}

	var agents []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	if len(agents) == 0 {
		fmt.Println("No agents registered.")
		return nil
	}

	for _, a := range agents {
		fmt.Printf("  %-16s %s\n", a.Name, a.Description)
	}
	return nil
}

func runAgentRun(cmd *cobra.Command, args []string) error {
	agentName := args[0]
	prompt := strings.Join(args[1:], " ")

	_, err := agentWebSocket(cmd, agentName, prompt, "")
	return err
}

func runAgentChat(cmd *cobra.Command, args []string) error {
	agentName := args[0]

	fmt.Printf("Starting chat with agent %q (Ctrl+C to exit)\n\n", agentName)

	var conversationID string
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/quit" || line == "/exit" {
			break
		}

		newConvID, err := agentWebSocket(cmd, agentName, line, conversationID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		if newConvID != "" {
			conversationID = newConvID
		}
		fmt.Println()
	}
	return nil
}

// agentWebSocket connects to the platform's WebSocket chat endpoint, sends a
// prompt with the given agent name, and streams the response to stdout.
// It returns the conversation ID (newly created or the one passed in).
func agentWebSocket(cmd *cobra.Command, agentName, prompt, conversationID string) (string, error) {
	baseURL := agentPlatformURL(cmd)
	token, err := agentAuthToken(cmd)
	if err != nil {
		return "", err
	}

	// Convert HTTP URL to WebSocket URL.
	u, err := url.Parse(baseURL + "/api/chat")
	if err != nil {
		return "", fmt.Errorf("parsing URL: %w", err)
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}

	header := http.Header{}
	header.Set("Cookie", "session="+token)

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		return "", fmt.Errorf("connecting to platform: %w", err)
	}
	defer conn.Close()

	// Send prompt with conversation_id to maintain session across turns.
	msg := map[string]string{
		"type":    "prompt",
		"content": prompt,
		"agent":   agentName,
	}
	if conversationID != "" {
		msg["conversation_id"] = conversationID
	}
	data, _ := json.Marshal(msg)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return "", fmt.Errorf("sending prompt: %w", err)
	}

	// Read events until done.
	resultConvID := conversationID
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return resultConvID, nil // connection closed
		}

		var ev struct {
			Type    string `json:"type"`
			Content string `json:"content"`
			Tool    string `json:"tool"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(raw, &ev); err != nil {
			continue
		}

		switch ev.Type {
		case "conversation_created":
			resultConvID = ev.Content
		case "text", "text_delta":
			fmt.Print(ev.Content)
		case "tool_start":
			fmt.Printf("\n[tool: %s] ", ev.Tool)
		case "tool_done":
			// Truncate long tool output for CLI display.
			output := ev.Content
			if len(output) > 200 {
				output = fmt.Sprintf("%s... [%d bytes total, showing first 200]", output[:200], len(output))
			}
			fmt.Printf("done (%s)\n", output)
		case "tool_error":
			fmt.Printf("error: %s\n", ev.Message)
		case "error":
			fmt.Fprintf(os.Stderr, "\nerror: %s\n", ev.Message)
			return resultConvID, nil
		case "done":
			fmt.Println()
			return resultConvID, nil
		}
	}
}
