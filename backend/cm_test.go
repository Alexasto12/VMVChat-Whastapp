package main

import "testing"

func TestParseInbound(t *testing.T) {
	// from como string
	m, ok := parseInbound([]byte(`{"from":"0031600000000","message":{"text":"hola"},"timeUtc":"2026-06-18T10:00:00"}`))
	if !ok || m.Chat != "0031600000000" || m.Text != "hola" || m.Direction != "in" {
		t.Fatalf("string from: %+v ok=%v", m, ok)
	}
	// from como objeto {number}
	m, ok = parseInbound([]byte(`{"from":{"number":"+34600"},"message":{"text":"hey"}}`))
	if !ok || m.Chat != "+34600" || m.TimeUTC == "" {
		t.Fatalf("object from: %+v ok=%v", m, ok)
	}
	// media real de CM: mediaUri + custom.message_type
	m, ok = parseInbound([]byte(`{"from":{"number":"+34600"},"message":{"text":"","media":{"mediaUri":"https://cdn.messaging.cm.com/x","contentType":"image/webp"},"custom":{"message_type":"sticker"}},"timeUtc":"2026-06-19T12:00:00"}`))
	if !ok || m.MediaURL != "https://cdn.messaging.cm.com/x" || m.MediaType != "sticker" {
		t.Fatalf("media: %+v ok=%v", m, ok)
	}
	// dedup: el mismo payload da el mismo external_id (estable entre reintentos)
	if m.ExternalID == "" {
		t.Fatal("external_id vacío")
	}
	m2, _ := parseInbound([]byte(`{"from":{"number":"+34600"},"message":{"text":"","media":{"mediaUri":"https://cdn.messaging.cm.com/x","contentType":"image/webp"},"custom":{"message_type":"sticker"}},"timeUtc":"2026-06-19T12:00:00"}`))
	if m2.ExternalID != m.ExternalID {
		t.Fatalf("external_id no estable: %s != %s", m2.ExternalID, m.ExternalID)
	}
	// body inválido o sin texto -> no parsea (el webhook igual responde 200)
	if _, ok := parseInbound([]byte(`{"from":"x","message":{}}`)); ok {
		t.Fatal("mensaje vacío no debería parsear")
	}
	if _, ok := parseInbound([]byte(`no-json`)); ok {
		t.Fatal("json inválido no debería parsear")
	}
}

func TestParseStatus(t *testing.T) {
	ref, st, ok := parseStatus([]byte(`{"reference":"abc","status":"Delivered"}`))
	if !ok || ref != "abc" || st != "delivered" {
		t.Fatalf("delivered: ref=%s st=%s ok=%v", ref, st, ok)
	}
	if _, st, _ := parseStatus([]byte(`{"reference":"abc","status":"Read"}`)); st != "read" {
		t.Fatalf("read: %s", st)
	}
	if _, st, _ := parseStatus([]byte(`{"reference":"abc","statusDescription":"message rejected"}`)); st != "failed" {
		t.Fatalf("failed: %s", st)
	}
	// sin reference no se puede correlacionar -> no parsea
	if _, _, ok := parseStatus([]byte(`{"status":"Delivered"}`)); ok {
		t.Fatal("status sin reference no debería parsear")
	}
	// un mensaje entrante (sin status) no debe colar como estado
	if _, _, ok := parseStatus([]byte(`{"reference":"x","foo":"bar"}`)); ok {
		t.Fatal("payload sin estado reconocible no debería parsear")
	}
}
