package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
)

type Auth struct {
	user, pass string
	db         *DB
}

func newAuth(db *DB) *Auth {
	u, p := os.Getenv("AUTH_USER"), os.Getenv("AUTH_PASSWORD")
	if u == "" || p == "" {
		log.Fatal("faltan variables de entorno: AUTH_USER y AUTH_PASSWORD son obligatorias")
	}
	return &Auth{user: u, pass: p, db: db}
}

func token() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (a *Auth) login(c *fiber.Ctx) error {
	var in struct{ User, Password string }
	if err := c.BodyParser(&in); err != nil {
		return fiber.ErrBadRequest
	}
	// comparación en tiempo constante para no filtrar el usuario/clave por timing
	okU := subtle.ConstantTimeCompare([]byte(in.User), []byte(a.user)) == 1
	okP := subtle.ConstantTimeCompare([]byte(in.Password), []byte(a.pass)) == 1
	if !okU || !okP {
		return fiber.ErrUnauthorized
	}
	t := token()
	if err := a.db.newSession(t); err != nil {
		return err
	}
	c.Cookie(&fiber.Cookie{
		Name:     "session",
		Value:    t,
		Path:     "/",
		HTTPOnly: true,
		SameSite: "Lax",
		MaxAge:   60 * 60 * 24 * 30, // 30 días
	})
	return c.JSON(fiber.Map{"ok": true})
}

func (a *Auth) logout(c *fiber.Ctx) error {
	a.db.deleteSession(c.Cookies("session"))
	c.ClearCookie("session")
	return c.JSON(fiber.Map{"ok": true})
}

func (a *Auth) ok(c *fiber.Ctx) bool {
	return a.db.sessionValid(c.Cookies("session"))
}

// middleware protege rutas: 401 si no hay sesión válida.
func (a *Auth) middleware(c *fiber.Ctx) error {
	if a.ok(c) {
		return c.Next()
	}
	return fiber.ErrUnauthorized
}
