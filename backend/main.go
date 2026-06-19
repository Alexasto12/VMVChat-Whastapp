package main

import (
	"crypto/subtle"
	"embed"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
)

//go:embed all:dist
var distFS embed.FS

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	apiKey := os.Getenv("CM_API_KEY")
	sender := os.Getenv("CM_SENDER")
	if apiKey == "" || sender == "" {
		log.Fatal("faltan variables de entorno: CM_API_KEY y CM_SENDER son obligatorias")
	}
	webhookToken := os.Getenv("WEBHOOK_TOKEN") // opcional pero recomendado en producción

	db := openDB("/data/chat.db")
	defer db.Close()
	go db.backupLoop("/data")

	hub := newHub()
	cm := &CM{apiKey: apiKey, sender: sender}
	auth := newAuth(db)

	app := fiber.New(fiber.Config{UnescapePath: true})

	// ── Públicas ─────────────────────────────────────────────────────────────
	app.Get("/health", func(c *fiber.Ctx) error {
		if err := db.Ping(); err != nil {
			return c.Status(503).JSON(fiber.Map{"status": "down"})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	})

	app.Post("/login", auth.login)
	app.Post("/logout", auth.logout)
	app.Get("/api/me", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"authed": auth.ok(c)})
	})

	// Webhook: lo protege un token en la query (?token=...), no la sesión (lo llama CM).
	app.Get("/webhook", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	app.Post("/webhook", func(c *fiber.Ctx) error {
		if webhookToken != "" &&
			subtle.ConstantTimeCompare([]byte(c.Query("token")), []byte(webhookToken)) != 1 {
			return c.SendStatus(http.StatusOK) // 200 igualmente para que CM no reintente, pero ignoramos
		}
		body := c.Body()
		if m, ok := parseInbound(body); ok {
			if !db.messageExists(m.ExternalID) {
				if err := db.save(&m); err == nil {
					hub.broadcast(m)
				}
			}
			return c.SendStatus(http.StatusOK)
		}
		if ref, st, ok := parseStatus(body); ok {
			if id, changed := db.updateStatusByRef(ref, st); changed {
				hub.broadcastStatus(id, st)
			}
			return c.SendStatus(http.StatusOK)
		}
		slog.Warn("webhook no parseado", "body", string(body))
		return c.SendStatus(http.StatusOK)
	})

	// ── Protegidas (sesión) ──────────────────────────────────────────────────
	api := app.Group("/api", auth.middleware)

	api.Get("/chats", func(c *fiber.Ctx) error {
		chats, err := db.listChats()
		if err != nil {
			return err
		}
		return c.JSON(chats)
	})

	api.Get("/chats/:phone", func(c *fiber.Ctx) error {
		before, _ := strconv.ParseInt(c.Query("before", "0"), 10, 64)
		limit, _ := strconv.Atoi(c.Query("limit", "30"))
		msgs, err := db.history(c.Params("phone"), before, limit)
		if err != nil {
			return err
		}
		return c.JSON(msgs)
	})

	api.Post("/chats/:phone/read", func(c *fiber.Ctx) error {
		if err := db.markRead(c.Params("phone")); err != nil {
			return err
		}
		return c.SendStatus(http.StatusNoContent)
	})

	api.Get("/search", func(c *fiber.Ctx) error {
		q := strings.TrimSpace(c.Query("q"))
		if len(q) < 2 {
			return c.JSON([]Message{})
		}
		res, err := db.search(q)
		if err != nil {
			return err
		}
		return c.JSON(res)
	})

	api.Get("/contacts/:phone", func(c *fiber.Ctx) error {
		contact, err := db.getContact(c.Params("phone"))
		if err != nil {
			return err
		}
		return c.JSON(contact)
	})

	api.Put("/contacts/:phone", func(c *fiber.Ctx) error {
		var in struct {
			Name string `json:"name"`
		}
		if err := c.BodyParser(&in); err != nil {
			return fiber.ErrBadRequest
		}
		if err := db.upsertContact(c.Params("phone"), in.Name); err != nil {
			return err
		}
		return c.JSON(Contact{Phone: c.Params("phone"), Name: in.Name})
	})

	api.Get("/templates", func(c *fiber.Ctx) error {
		tpls, err := db.listTemplates()
		if err != nil {
			return err
		}
		return c.JSON(tpls)
	})

	api.Post("/templates", func(c *fiber.Ctx) error {
		var t DBTemplate
		if err := c.BodyParser(&t); err != nil || t.Name == "" || t.Namespace == "" {
			return fiber.ErrBadRequest
		}
		if err := db.saveTemplate(t); err != nil {
			return err
		}
		return c.JSON(t)
	})

	api.Delete("/templates/:name", func(c *fiber.Ctx) error {
		if err := db.deleteTemplate(c.Params("name")); err != nil {
			return err
		}
		return c.SendStatus(http.StatusNoContent)
	})

	// ── Envío (protegido) ────────────────────────────────────────────────────
	app.Post("/send", auth.middleware, func(c *fiber.Ctx) error {
		var in struct{ To, Text string }
		if err := c.BodyParser(&in); err != nil || in.To == "" || in.Text == "" {
			return fiber.ErrBadRequest
		}
		ref := token()
		m := Message{Chat: in.To, Direction: "out", Text: in.Text, Status: "sent", TimeUTC: nowUTC(), Reference: ref}
		if err := cm.send(in.To, in.Text, ref); err != nil {
			slog.Error("envío falló", "err", err)
			m.Status = "failed"
		}
		if err := db.save(&m); err != nil {
			return err
		}
		hub.broadcast(m)
		return c.JSON(m)
	})

	app.Post("/send-template", auth.middleware, func(c *fiber.Ctx) error {
		var in struct {
			To, Namespace, Name, Lang, Body string
			Params                          []string
		}
		if err := c.BodyParser(&in); err != nil || in.To == "" || in.Name == "" {
			return fiber.ErrBadRequest
		}
		if in.Lang == "" {
			in.Lang = "es"
		}
		shown := in.Body
		if shown == "" {
			shown = in.Name
		}
		for i, p := range in.Params {
			shown = strings.ReplaceAll(shown, "{{"+strconv.Itoa(i+1)+"}}", p)
		}
		ref := token()
		m := Message{Chat: in.To, Direction: "out", Text: shown, MediaType: "template", Status: "sent", TimeUTC: nowUTC(), Reference: ref}
		if err := cm.sendTemplate(in.To, in.Namespace, in.Name, in.Lang, ref, in.Params); err != nil {
			slog.Error("template envío falló", "err", err)
			m.Status = "failed"
		}
		if err := db.save(&m); err != nil {
			return err
		}
		hub.broadcast(m)
		return c.JSON(m)
	})

	// ── Media proxy (protegido; las <img> mandan la cookie en same-origin) ─────
	app.Get("/media", auth.middleware, func(c *fiber.Ctx) error {
		u := c.Query("url")
		if !strings.HasPrefix(u, "https://") || !strings.Contains(u, "cm.com") {
			return fiber.ErrBadRequest
		}
		req, _ := http.NewRequest("GET", u, nil)
		req.Header.Set("X-CM-PRODUCTTOKEN", apiKey)
		resp, err := httpClient.Do(req)
		if err != nil {
			return fiber.ErrBadGateway
		}
		defer resp.Body.Close()
		if ct := resp.Header.Get("Content-Type"); ct != "" {
			c.Set("Content-Type", ct)
		}
		c.Set("Cache-Control", "public, max-age=86400")
		body, _ := io.ReadAll(resp.Body)
		return c.Send(body)
	})

	// ── WebSocket (protegido por cookie) + SPA ────────────────────────────────
	app.Use("/ws", func(c *fiber.Ctx) error {
		if !auth.ok(c) {
			return fiber.ErrUnauthorized
		}
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws", websocket.New(hub.handle))

	sub, _ := fs.Sub(distFS, "dist")
	app.Use("/", filesystem.New(filesystem.Config{Root: http.FS(sub), Index: "index.html"}))

	slog.Info("VMVChat escuchando", "addr", ":8080")
	log.Fatal(app.Listen(":8080"))
}
