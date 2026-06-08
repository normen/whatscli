// this file implements the optional OpenAI-powered auto-reply bot
package messages

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/normen/whatscli/config"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

const openAIEndpoint = "https://api.openai.com/v1/chat/completions"

// defaultBotModel is used when no model is configured.
const defaultBotModel = "gpt-4o-mini"

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Stream   bool            `json:"stream,omitempty"`
}

// streamChunk is one server-sent event from the streaming chat completions API.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// defaultMemoryMessages is how many recent messages are sent as context when unset.
const defaultMemoryMessages = 10

// askOpenAIStream streams a chat completion from OpenAI, invoking onDelta for each
// piece of text as it arrives. It honours ctx: cancelling ctx aborts the request
// mid-stream (this is how an incoming message interrupts an in-flight reply).
func askOpenAIStream(ctx context.Context, apiKey, model, systemPrompt string, history []openAIMessage, onDelta func(string)) error {
	if apiKey == "" {
		return errors.New("no OpenAI API key available")
	}
	if model == "" {
		model = defaultBotModel
	}

	msgs := make([]openAIMessage, 0, len(history)+1)
	if systemPrompt != "" {
		msgs = append(msgs, openAIMessage{Role: "system", Content: systemPrompt})
	}
	msgs = append(msgs, history...)

	payload, err := json.Marshal(openAIRequest{Model: model, Messages: msgs, Stream: true})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIEndpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("openai error: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			line = strings.TrimSpace(line)
			if data, ok := strings.CutPrefix(line, "data:"); ok {
				data = strings.TrimSpace(data)
				if data == "[DONE]" {
					return nil
				}
				var chunk streamChunk
				if jsonErr := json.Unmarshal([]byte(data), &chunk); jsonErr == nil {
					if chunk.Error != nil {
						return fmt.Errorf("openai error: %s", chunk.Error.Message)
					}
					for _, c := range chunk.Choices {
						if c.Delta.Content != "" {
							onDelta(c.Delta.Content)
						}
					}
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			// a cancelled ctx surfaces here as a read error; report it as such
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return err
		}
	}
}

// chat commands the bot understands.
const (
	keyCommand = "/key" // register a per-chat OpenAI key
	aiCommand  = "/ai"  // start a free-flowing AI conversation in this chat
	endCommand = "/end" // leave the AI conversation
)

// botThinkingPlaceholder is sent immediately when the bot starts working, so the
// user gets instant feedback ("the AI is thinking") instead of silence. It is
// then edited in place as the streamed answer arrives.
const botThinkingPlaceholder = "🤖 _digitando..._"

// editThrottle is the minimum delay between streamed message edits, to avoid
// hammering WhatsApp with an edit on every single token.
const editThrottle = 1200 * time.Millisecond

// aiChatState tracks the "/ai" conversation mode for a single chat.
type aiChatState struct {
	active bool               // whether the chat is in free-flowing "/ai" mode
	cancel context.CancelFunc // cancels the in-flight generation, nil when idle
	gen    uint64             // generation counter; a new message supersedes older ones
}

// isChatCommand reports whether text is the given command, optionally followed by
// arguments (e.g. "/key sk-...").
func isChatCommand(text, cmd string) bool {
	return text == cmd || strings.HasPrefix(text, cmd+" ")
}

// maybeReplyWithBot decides what the AI bot should do with an incoming message:
// handle a chat command (/key, /ai, /end), answer it (in "/ai" mode or when the
// trigger prefix is present), or ignore it. Replies are streamed back, editing a
// placeholder message in place so the user sees the answer appear progressively.
func (sm *SessionManager) maybeReplyWithBot(msg Message) {
	bot := config.Config.Bot
	if bot == nil || !bot.Enabled {
		return
	}
	// only act on incoming text messages in the single configured chat
	if msg.FromMe || msg.Kind != MessageKindText || strings.TrimSpace(msg.Text) == "" {
		return
	}
	if bot.ChatId == "" || msg.ChatId != bot.ChatId {
		return
	}

	trimmed := strings.TrimSpace(msg.Text)
	switch {
	case isChatCommand(trimmed, keyCommand):
		sm.handleKeyCommand(msg.ChatId, trimmed)
		return
	case isChatCommand(trimmed, aiCommand):
		sm.startAISession(msg.ChatId)
		return
	case isChatCommand(trimmed, endCommand):
		sm.endAISession(msg.ChatId)
		return
	}

	// In "/ai" mode every message gets answered. Otherwise fall back to the
	// trigger-prefix behaviour: stay quiet until addressed with the prefix.
	if !sm.aiSessionActive(msg.ChatId) {
		if prefix := bot.TriggerPrefix; prefix != "" {
			if !strings.HasPrefix(trimmed, prefix) {
				return
			}
			// ignore a bare "@" with no actual question after it
			if strings.TrimSpace(strings.TrimPrefix(trimmed, prefix)) == "" {
				return
			}
		}
	}

	sm.replyWithBotStreaming(msg)
}

// startAISession puts a chat into free-flowing "/ai" mode and confirms it.
func (sm *SessionManager) startAISession(chatID string) {
	sm.aiChatsLock.Lock()
	st := sm.aiChats[chatID]
	if st == nil {
		st = &aiChatState{}
		sm.aiChats[chatID] = st
	}
	st.active = true
	sm.aiChatsLock.Unlock()

	jid, err := types.ParseJID(chatID)
	if err == nil {
		sm.botSendText(jid, chatID, "🤖 Modo IA ativado. Mando uma resposta para cada mensagem sua — é só escrever. Envie /end para encerrar.")
	}
}

// endAISession leaves "/ai" mode and cancels any in-flight generation.
func (sm *SessionManager) endAISession(chatID string) {
	sm.aiChatsLock.Lock()
	st := sm.aiChats[chatID]
	if st != nil {
		st.active = false
		if st.cancel != nil {
			st.cancel()
			st.cancel = nil
		}
	}
	sm.aiChatsLock.Unlock()

	jid, err := types.ParseJID(chatID)
	if err == nil {
		sm.botSendText(jid, chatID, "🤖 Modo IA encerrado. Quando quiser voltar, mande /ai.")
	}
}

func (sm *SessionManager) aiSessionActive(chatID string) bool {
	sm.aiChatsLock.Lock()
	defer sm.aiChatsLock.Unlock()
	st := sm.aiChats[chatID]
	return st != nil && st.active
}

// beginAIGeneration cancels any in-flight generation for the chat and returns a
// fresh context plus its generation number. This is what makes a new incoming
// message interrupt the bot's previous (possibly still streaming) answer.
func (sm *SessionManager) beginAIGeneration(chatID string) (context.Context, uint64) {
	sm.aiChatsLock.Lock()
	defer sm.aiChatsLock.Unlock()
	st := sm.aiChats[chatID]
	if st == nil {
		st = &aiChatState{}
		sm.aiChats[chatID] = st
	}
	if st.cancel != nil {
		st.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	st.cancel = cancel
	st.gen++
	return ctx, st.gen
}

// finishAIGeneration clears the stored cancel func, but only if a newer
// generation has not already replaced it.
func (sm *SessionManager) finishAIGeneration(chatID string, gen uint64) {
	sm.aiChatsLock.Lock()
	defer sm.aiChatsLock.Unlock()
	if st := sm.aiChats[chatID]; st != nil && st.gen == gen {
		st.cancel = nil
	}
}

// replyWithBotStreaming kicks off a streamed reply to msg in a background
// goroutine, cancelling any reply that is already in flight for this chat.
func (sm *SessionManager) replyWithBotStreaming(msg Message) {
	bot := config.Config.Bot

	// pick the API key, model and memory window: a user-supplied key unlocks a
	// bigger model and a larger context window ("boost").
	apiKey := sm.getUserKey(msg.ChatId)
	model := bot.Model
	memory := bot.MemoryMessages
	if apiKey != "" {
		model = bot.BoostModel
		memory = bot.BoostMemoryMessages
	} else {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if memory <= 0 {
		memory = defaultMemoryMessages
	}

	history := sm.recentBotHistory(msg.ChatId, memory)

	// let the model know its memory is limited so it doesn't claim to recall older messages
	systemPrompt := fmt.Sprintf(
		"%s\n\n(Context note: you can only see the last %d messages of this conversation. Anything older is not available to you.)",
		bot.SystemPrompt, memory,
	)

	ctx, gen := sm.beginAIGeneration(msg.ChatId)
	go sm.runStreamingReply(ctx, gen, msg.ChatId, apiKey, model, systemPrompt, history)
}

// runStreamingReply performs one streamed reply: it posts a placeholder message
// (the "thinking" indicator), then edits it in place as tokens arrive, and
// finalises it on completion, interruption or error.
func (sm *SessionManager) runStreamingReply(ctx context.Context, gen uint64, chatID, apiKey, model, systemPrompt string, history []openAIMessage) {
	defer sm.finishAIGeneration(chatID, gen)

	receiver, err := types.ParseJID(chatID)
	if err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("bot: invalid JID: %v", err))
		return
	}

	// instant feedback so the user isn't left talking to a ghost
	sm.sendComposing(receiver)
	msgID, err := sm.botSendText(receiver, chatID, botThinkingPlaceholder)
	if err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("bot: %v", err))
		return
	}

	var full strings.Builder
	var lastEdit time.Time
	flush := func(force bool) {
		if full.Len() == 0 {
			return
		}
		if !force && time.Since(lastEdit) < editThrottle {
			return
		}
		sm.botEditText(receiver, chatID, msgID, full.String())
		sm.sendComposing(receiver) // keep the "typing" indicator alive
		lastEdit = time.Now()
	}

	streamErr := askOpenAIStream(ctx, apiKey, model, systemPrompt, history, func(delta string) {
		full.WriteString(delta)
		flush(false)
	})

	sm.sendPaused(receiver)

	switch {
	case ctx.Err() != nil:
		// interrupted by a newer message or by /end
		final := strings.TrimSpace(full.String())
		if final == "" {
			final = "⏹️ _(interrompido)_"
		} else {
			final += "\n\n⏹️ _(interrompido)_"
		}
		sm.botEditText(receiver, chatID, msgID, final)
	case streamErr != nil:
		sm.uiHandler.PrintError(fmt.Errorf("bot: %v", streamErr))
		sm.botEditText(receiver, chatID, msgID, "⚠️ _não consegui responder agora. Tente de novo._")
	default:
		final := strings.TrimSpace(full.String())
		if final == "" {
			final = "🤖 _(sem resposta)_"
		}
		sm.botEditText(receiver, chatID, msgID, final)
	}
}

// botSendText sends a new text message, records it locally and refreshes the UI,
// returning the WhatsApp message id so it can later be edited.
func (sm *SessionManager) botSendText(receiver types.JID, chatID, text string) (types.MessageID, error) {
	if sm.client == nil || !sm.client.IsConnected() {
		return "", errors.New("not connected to WhatsApp")
	}
	raw := &waProto.Message{Conversation: proto.String(text)}
	resp, err := sm.client.SendMessage(context.Background(), receiver, raw)
	if err != nil {
		return "", err
	}
	newMsg := sm.outgoingMessageFromSendResponse(resp, chatID, raw, MessageKindText, text, "", "")
	sm.db.AddMessage(newMsg, false)
	if sm.getCurrentReceiver() == chatID {
		sm.uiHandler.NewMessage(newMsg)
	}
	sm.uiHandler.SetChats(sm.db.GetChatIds())
	return resp.ID, nil
}

// botEditText edits a previously sent message in place (used for streaming) and
// reflects the new text in the local store and UI.
func (sm *SessionManager) botEditText(receiver types.JID, chatID string, id types.MessageID, text string) {
	if sm.client == nil || !sm.client.IsConnected() {
		return
	}
	edit := sm.client.BuildEdit(receiver, id, &waProto.Message{Conversation: proto.String(text)})
	if _, err := sm.client.SendMessage(context.Background(), receiver, edit); err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("bot edit: %v", err))
		return
	}
	if _, ok := sm.db.UpdateMessageText(string(id), text); ok && sm.getCurrentReceiver() == chatID {
		sm.uiHandler.NewScreen(sm.db.GetMessages(chatID))
	}
}

// sendComposing / sendPaused toggle the WhatsApp "typing..." indicator. Errors
// are non-fatal (the placeholder message already signals activity).
func (sm *SessionManager) sendComposing(receiver types.JID) {
	if sm.client == nil {
		return
	}
	_ = sm.client.SendChatPresence(context.Background(), receiver, types.ChatPresenceComposing, types.ChatPresenceMediaText)
}

func (sm *SessionManager) sendPaused(receiver types.JID) {
	if sm.client == nil {
		return
	}
	_ = sm.client.SendChatPresence(context.Background(), receiver, types.ChatPresencePaused, types.ChatPresenceMediaText)
}

// handleKeyCommand processes the "/key" chat command: setting, clearing, or
// rejecting a user-supplied API key, and replies to the user accordingly.
func (sm *SessionManager) handleKeyCommand(chatID, text string) {
	arg := strings.TrimSpace(strings.TrimPrefix(text, keyCommand))
	var reply string

	switch {
	case arg == "":
		reply = "Use \"/key sk-...\" para usar sua própria conta OpenAI (contexto maior). " +
			"Use \"/key reset\" para voltar ao padrão. ⚠️ No momento só funcionam keys da OpenAI."
	case arg == "reset" || arg == "off" || arg == "clear":
		sm.clearUserKey(chatID)
		reply = "Sua key foi removida. Voltando ao modo padrão."
	case strings.HasPrefix(arg, "sk-"):
		sm.setUserKey(chatID, arg)
		reply = fmt.Sprintf(
			"✅ Sua key da OpenAI foi ativada para esta conversa: modelo %s e memória das últimas %d mensagens. "+
				"⚠️ Por segurança, apague a mensagem que contém a key — ela fica visível no histórico do WhatsApp.",
			config.Config.Bot.BoostModel, config.Config.Bot.BoostMemoryMessages,
		)
	default:
		reply = "⚠️ No momento só funcionam keys da OpenAI (começam com \"sk-\"). Sua key não foi aceita."
	}

	sm.CommandChannel <- Command{Name: "send", Params: []string{chatID, reply}}
}

func (sm *SessionManager) getUserKey(chatID string) string {
	sm.userKeysLock.RLock()
	defer sm.userKeysLock.RUnlock()
	return sm.userKeys[chatID]
}

func (sm *SessionManager) setUserKey(chatID, key string) {
	sm.userKeysLock.Lock()
	defer sm.userKeysLock.Unlock()
	sm.userKeys[chatID] = key
}

func (sm *SessionManager) clearUserKey(chatID string) {
	sm.userKeysLock.Lock()
	defer sm.userKeysLock.Unlock()
	delete(sm.userKeys, chatID)
}

// recentBotHistory returns up to `limit` of the most recent text messages of a
// chat as OpenAI chat messages, mapping our own messages to the "assistant" role
// and incoming messages to the "user" role.
func (sm *SessionManager) recentBotHistory(chatID string, limit int) []openAIMessage {
	all := sm.db.GetMessages(chatID) // sorted oldest -> newest

	// collect the last `limit` text messages
	picked := make([]Message, 0, limit)
	for i := len(all) - 1; i >= 0 && len(picked) < limit; i-- {
		m := all[i]
		if m.Kind != MessageKindText || strings.TrimSpace(m.Text) == "" {
			continue
		}
		// never feed "/key ..." command messages (which may contain a secret) to the model
		if t := strings.TrimSpace(m.Text); t == keyCommand || strings.HasPrefix(t, keyCommand+" ") {
			continue
		}
		picked = append(picked, m)
	}

	// reverse back into chronological order
	prefix := config.Config.Bot.TriggerPrefix
	history := make([]openAIMessage, 0, len(picked))
	for i := len(picked) - 1; i >= 0; i-- {
		role := "user"
		if picked[i].FromMe {
			role = "assistant"
		}
		content := picked[i].Text
		// strip the trigger prefix (e.g. "@") so the model sees the clean question
		if prefix != "" {
			content = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(content), prefix))
		}
		history = append(history, openAIMessage{Role: role, Content: content})
	}
	return history
}
