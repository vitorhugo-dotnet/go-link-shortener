package middleware

import "github.com/gofiber/fiber/v3"

// OriginIsolation enables browser APIs, including WebMCP, that require an
// origin-isolated document.
func OriginIsolation(originTrialToken string) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Set("Cross-Origin-Opener-Policy", "same-origin")
		c.Set("Cross-Origin-Embedder-Policy", "require-corp")
		if originTrialToken != "" {
			c.Set("Origin-Trial", originTrialToken)
		}
		return c.Next()
	}
}
