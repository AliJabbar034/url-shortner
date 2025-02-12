package routes

import (
	"github.com/alijabbar034/url-shortner/database"
	"github.com/alijabbar034/url-shortner/helpers"
	valid "github.com/asaskevich/govalidator/v11"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"os"
	"strconv"
	"time"
)

type request struct {
	URL         string        `json:"url"`
	CustomShort string        `json:"custom_short"`
	Expiry      time.Duration `json:"expiry"`
}
type response struct {
	URL            string        `json:"url"`
	CustomShort    string        `json:"custom_short"`
	Expiry         time.Duration `json:"expiry"`
	XRateRemaining int           `json:"x_rate_remaining"`
	XRateReset     int           `json:"x_rate_reset"`
}

func ShortenURL(c *fiber.Ctx) error {
	body := new(request)
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid data",
		})
	}
	r2 := database.CreateClient(1)
	defer r2.Close()
	value, err := r2.Get(database.CTX, c.IP()).Result()
	if err == redis.Nil {
		_ = r2.Set(database.CTX, c.IP(), os.Getenv("API_QUOTA"), 30*60*time.Second).Err()

	} else {
		value, _ = r2.Get(database.CTX, c.IP()).Result()
		varInt, _ := strconv.Atoi(value)
		if varInt <= 0 {
			limit, _ := r2.TTL(database.CTX, c.IP()).Result()
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error":      "Rate limit exceeded",
				"rate_limit": limit / time.Nanosecond / time.Second,
			})
		}
	}
	if !valid.IsURL(body.URL) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid URL",
		})
	}

	if !helpers.RemoveDomainError(body.URL) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid URL",
		})
	}
	body.URL = helpers.EnforceHTTP(body.URL)
	var id string
	if body.CustomShort == "" {
		id = uuid.New().String()[:6]
	} else {
		id = body.CustomShort
	}
	r := database.CreateClient(0)

	val, _ := r.Get(database.CTX, id).Result()
	if val != "" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "URL Custom Short is already used",
		})
	}
	if body.Expiry == 0 {
		body.Expiry = 24
	}
	err = r.Set(database.CTX, id, body.URL, body.Expiry*3600*time.Second).Err()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to set URL Custom Short",
		})
	}
	defer r.Close()
	defer r2.Decr(database.CTX, c.IP())
	return nil

}
