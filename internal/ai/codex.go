package ai

// CodexRequiredEnv returns the list of environment variables required by Codex.
func CodexRequiredEnv() []string {
	return []string{"OPENAI_API_KEY"}
}
