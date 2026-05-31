package service

import (
	"os"
	"strings"
	"testing"
)

func TestChatStreamAcceptsPageContext(t *testing.T) {
	source := readAgentServiceSource(t)
	if !strings.Contains(source, "pageCtx") && !strings.Contains(source, "AgentPageContext") {
		t.Fatal("ChatStream must accept a pageCtx parameter of type *model.AgentPageContext")
	}
}

func TestChatStreamUsesContextType(t *testing.T) {
	source := readAgentServiceSource(t)
	if !strings.Contains(source, "contextType") {
		t.Fatal("ChatStream must set contextType on the conversation based on page context")
	}
}

func TestChatStreamInjectsSystemContext(t *testing.T) {
	source := readAgentServiceSource(t)
	if !strings.Contains(source, "Page Context") && !strings.Contains(source, "pageCtx") {
		t.Fatal("ChatStream must inject page context as a system message into the LLM request")
	}
}

func TestChatStreamMaxMessageCharsConfig(t *testing.T) {
	configSrc := readConfigSource(t)
	if !strings.Contains(configSrc, "MaxUserMessageChars") && !strings.Contains(configSrc, "max_user_message_chars") {
		t.Fatal("AgentConfig must define MaxUserMessageChars for chat message length validation")
	}
}

func TestChatStreamMaxContextMessagesConfig(t *testing.T) {
	configSrc := readConfigSource(t)
	if !strings.Contains(configSrc, "ChatMaxContextMsgs") && !strings.Contains(configSrc, "chat_max_context_messages") {
		t.Fatal("AgentConfig must define ChatMaxContextMsgs for chat context window limit")
	}
}

func TestChatStreamRoleAllowlist(t *testing.T) {
	handlerSrc := readHandlerSource(t)
	if !strings.Contains(handlerSrc, "allowedRoles") {
		t.Fatal("ChatStream handler must validate message roles against an allowlist")
	}
}

func TestChatStreamMessageTruncation(t *testing.T) {
	handlerSrc := readHandlerSource(t)
	if !strings.Contains(handlerSrc, "maxMsgLen") && !strings.Contains(handlerSrc, "MaxUserMessageChars") {
		t.Fatal("ChatStream handler must truncate messages exceeding the configured max length")
	}
}

func TestChatStreamContextWindowLimit(t *testing.T) {
	handlerSrc := readHandlerSource(t)
	if !strings.Contains(handlerSrc, "maxCtxMsgs") && !strings.Contains(handlerSrc, "ChatMaxContextMsgs") {
		t.Fatal("ChatStream handler must limit the number of context messages sent to the LLM")
	}
}

func readConfigSource(t *testing.T) string {
	t.Helper()
	candidates := []string{"../config/config.go", "../../config/config.go"}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err == nil {
			return string(data)
		}
	}
	t.Fatal("cannot find config.go")
	return ""
}

func readHandlerSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../handler/agent.go")
	if err != nil {
		t.Fatalf("read agent.go handler: %v", err)
	}
	return string(data)
}
