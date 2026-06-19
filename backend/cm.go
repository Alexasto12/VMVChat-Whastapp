package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type CM struct {
	apiKey string
	sender string
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

func doRequest(req *http.Request) ([]byte, error) {
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("CM %d: %s", resp.StatusCode, b)
	}
	return b, nil
}

func (cm *CM) send(to, text, ref string) error {
	body := map[string]any{
		"messages": map[string]any{
			"authentication": map[string]any{"productToken": cm.apiKey},
			"msg": []map[string]any{{
				"from":            cm.sender,
				"to":              []map[string]string{{"number": to}},
				"body":            map[string]string{"type": "auto", "content": text},
				"allowedChannels": []string{"WhatsApp"},
				"reference":       ref, // CM lo devuelve en los reportes de estado
			}},
		},
	}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "https://gw.messaging.cm.com/v1.0/message", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CM-PRODUCTTOKEN", cm.apiKey)
	_, err := doRequest(req)
	return err
}

func (cm *CM) sendTemplate(to, namespace, name, langCode, ref string, params []string) error {
	var bodyParams []map[string]string
	for _, p := range params {
		bodyParams = append(bodyParams, map[string]string{"type": "text", "text": p})
	}
	components := []map[string]any{}
	if len(bodyParams) > 0 {
		components = append(components, map[string]any{"type": "body", "parameters": bodyParams})
	}
	body := map[string]any{
		"messages": map[string]any{
			"authentication": map[string]any{"productToken": cm.apiKey},
			"msg": []map[string]any{{
				"from":            cm.sender,
				"to":              []map[string]string{{"number": to}},
				"body":            map[string]string{"type": "auto", "content": name},
				"allowedChannels": []string{"WhatsApp"},
				"reference":       ref,
				"richContent": map[string]any{
					"conversation": []map[string]any{{
						"template": map[string]any{
							"whatsapp": map[string]any{
								"namespace":    namespace,
								"element_name": name,
								"language":     map[string]string{"policy": "deterministic", "code": langCode},
								"components":   components,
							},
						},
					}},
				},
			}},
		},
	}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "https://gw.messaging.cm.com/v1.0/message", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CM-PRODUCTTOKEN", cm.apiKey)
	_, err := doRequest(req)
	return err
}

// parseInbound acepta texto, media (imagen, sticker, vídeo, audio, documento) y eventos de status.
// ponytail: log del body raw en webhook si algo no parsea — ajusta desde ahí.
func parseInbound(raw []byte) (Message, bool) {
	var p struct {
		From    json.RawMessage `json:"from"`
		Message struct {
			Text  string `json:"text"`
			Media *struct {
				MediaURI    string `json:"mediaUri"`
				ContentType string `json:"contentType"`
			} `json:"media"`
			Custom struct {
				MessageType string `json:"message_type"`
			} `json:"custom"`
		} `json:"message"`
		TimeUTC string `json:"timeUtc"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return Message{}, false
	}
	from := parseFrom(p.From)
	if from == "" {
		return Message{}, false
	}

	hasMedia := p.Message.Media != nil && p.Message.Media.MediaURI != ""
	if p.Message.Text == "" && !hasMedia {
		return Message{}, false // status report sin contenido
	}

	ts := p.TimeUTC
	if ts == "" {
		ts = nowUTC()
	}
	m := Message{Chat: from, Direction: "in", Text: p.Message.Text, Status: "received", TimeUTC: ts}

	if hasMedia {
		m.MediaURL = p.Message.Media.MediaURI
		m.MediaType = mediaType(p.Message.Custom.MessageType, p.Message.Media.ContentType)
	}
	m.ExternalID = fingerprint(m.Chat, p.TimeUTC, m.Text, m.MediaURL)
	return m, true
}

// parseStatus interpreta los reportes de estado de CM (delivered/read/failed) por reference.
// ponytail: asume reference echo + un campo de estado; verifica contra un webhook de estado real (se loguea el raw si no casa).
func parseStatus(raw []byte) (ref, status string, ok bool) {
	var p struct {
		Reference         string `json:"reference"`
		Status            string `json:"status"`
		StatusDescription string `json:"statusDescription"`
		MessageStatus     string `json:"messageStatus"`
	}
	if json.Unmarshal(raw, &p) != nil || p.Reference == "" {
		return "", "", false
	}
	s := strings.ToLower(p.Status + " " + p.StatusDescription + " " + p.MessageStatus)
	switch {
	case strings.Contains(s, "read"):
		status = "read"
	case strings.Contains(s, "deliver"):
		status = "delivered"
	case strings.Contains(s, "fail"), strings.Contains(s, "reject"), strings.Contains(s, "error"), strings.Contains(s, "undeliver"):
		status = "failed"
	case strings.Contains(s, "sent"), strings.Contains(s, "accept"):
		status = "sent"
	default:
		return "", "", false
	}
	return p.Reference, status, true
}

// mediaType usa custom.message_type de CM (audio/image/sticker/video/document); cae al contentType si falta.
func mediaType(kind, contentType string) string {
	switch kind {
	case "image", "sticker", "video", "audio", "document":
		return kind
	}
	switch {
	case strings.HasPrefix(contentType, "image/webp"):
		return "sticker"
	case strings.HasPrefix(contentType, "image/"):
		return "image"
	case strings.HasPrefix(contentType, "video/"):
		return "video"
	case strings.HasPrefix(contentType, "audio/"):
		return "audio"
	default:
		return "document"
	}
}

func parseFrom(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil && s != "" {
		return s
	}
	var o struct{ Number string `json:"number"` }
	if json.Unmarshal(raw, &o) == nil {
		return o.Number
	}
	return ""
}
