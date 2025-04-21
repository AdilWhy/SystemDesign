package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type User struct {
	ClientSecret string
	Scopes       []string
	Tokens       []string
}

type TokenInfo struct {
	ClientID       string
	AccessScope    string
	ExpirationTime time.Time
}

// TokenBucket represents a token bucket rate limiter
type TokenBucket struct {
	tokens         float64
	capacity       float64
	fillRate       float64
	lastRefillTime time.Time
	mutex          sync.Mutex
}

// RateLimiter manages rate limiting for different clients
type RateLimiter struct {
	limiters  sync.Map // map[string]*TokenBucket
	capacity  float64
	fillRate  float64
	globalMux sync.Mutex
}

var users sync.Map
var tokens sync.Map

// rateLimiter is the global rate limiter instance
var rateLimiter *RateLimiter

var dbconn *pgxpool.Pool
var (
	ErrNoToken      error = errors.New("nonexistent token")
	ErrTokenExpired error = errors.New("token expired")
	ErrRateLimited  error = errors.New("rate limit exceeded, please try again later")
)

func GetAllUsers() {
	for {
		rows, err := dbconn.Query(context.Background(), "select * from public.user")
		if err != nil {
			rows.Close()
			if err == pgx.ErrNoRows {
				continue
			}
			log.Fatal("Error getting all users at startup: ", err)
		}
		type ID_USER struct {
			id   string
			user User
		}
		usrs := make([]ID_USER, 0, 1000)
		for rows.Next() {
			var user User
			var id string
			err := rows.Scan(&id, &user.ClientSecret, &user.Scopes)
			if err != nil {
				log.Fatal("Error scanning user at startup: ", err)
			}
			user.Tokens = make([]string, len(user.Scopes))
			usrs = append(usrs, ID_USER{id, user})
		}
		rows.Close()
		if len(usrs) == 1000 {
			for i := range usrs {
				users.Store(usrs[i].id, usrs[i].user)
			}
			log.Println("Successfull getting all users at startup")
			return
		}
		time.Sleep(time.Second)
	}
}

func get_token(context context.Context, client_id string, scope string) string {
	row := dbconn.QueryRow(context, "select access_token, expiration_time from token where client_id=$1 and access_scope=$2", client_id, scope)
	var token string
	var exp_time time.Time
	err := row.Scan(&token, &exp_time)
	if err == pgx.ErrNoRows {
		return ""
	}
	if err != nil {
		log.Fatal("Error getting token: ", err)
	}
	if exp_time.Before(time.Now()) {
		dbconn.Exec(context, "delete from token where access_token=$1", token)
		return ""
	}
	return token
}

func AddToken(context context.Context, client_id string, scope string) string {
	// Check local cache
	if item, ok := users.Load(client_id); ok {
		user := item.(User)
		for i := range user.Tokens {
			if user.Scopes[i] == scope && user.Tokens[i] != "" {
				return user.Tokens[i]
			}
		}
	}

	token := get_token(context, client_id, scope)
	if token != "" {
		return token
	}
	row := dbconn.QueryRow(context, "insert into token(client_id, access_scope) VALUES($1, $2) returning access_token", client_id, scope)
	err := row.Scan(&token)
	if err != nil {
		return get_token(context, client_id, scope)
	}
	return token
}

func CheckToken(context context.Context, token string) (string, string, error) {
	// check local cache
	if item, ok := tokens.Load(token); ok {
		token_info := item.(TokenInfo)
		id := token_info.ClientID
		scope := token_info.AccessScope
		tim := token_info.ExpirationTime
		if tim.After(time.Now()) {
			return id, scope, nil
		}
		tokens.Delete(token)
	}
	row := dbconn.QueryRow(context, "select client_id, access_scope, expiration_time from token where access_token=$1", token)
	var id, scope string
	var exp_time time.Time
	err := row.Scan(&id, &scope, &exp_time)
	if err == pgx.ErrNoRows {
		return "", "", ErrNoToken
	}
	if exp_time.Before(time.Now()) {
		return "", "", ErrTokenExpired
	}
	tokens.Store(token, TokenInfo{id, scope, exp_time})
	// Set new token for (user, scope)
	if item, ok := users.Load(id); ok {
		user := item.(User)
		for i := range user.Tokens {
			if user.Scopes[i] == scope {
				user.Tokens[i] = token
				break
			}
		}
	}
	return id, scope, nil
}

// NewRateLimiter creates a new rate limiter with specified capacity and rate per second
func NewRateLimiter(capacity float64, ratePerSecond float64) *RateLimiter {
	return &RateLimiter{
		limiters:  sync.Map{},
		capacity:  capacity,
		fillRate:  ratePerSecond,
		globalMux: sync.Mutex{},
	}
}

// getLimiter gets or creates a token bucket for a client
func (rl *RateLimiter) getLimiter(clientID string) *TokenBucket {
	// Check if limiter already exists for this client
	if limiter, exists := rl.limiters.Load(clientID); exists {
		return limiter.(*TokenBucket)
	}

	// Create a new limiter if it doesn't exist
	rl.globalMux.Lock()
	defer rl.globalMux.Unlock()

	// Double-check after acquiring the lock
	if limiter, exists := rl.limiters.Load(clientID); exists {
		return limiter.(*TokenBucket)
	}

	// Create a new token bucket
	bucket := &TokenBucket{
		tokens:         rl.capacity,
		capacity:       rl.capacity,
		fillRate:       rl.fillRate,
		lastRefillTime: time.Now(),
		mutex:          sync.Mutex{},
	}
	rl.limiters.Store(clientID, bucket)
	return bucket
}

// refill refills the token bucket based on elapsed time
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsedTime := now.Sub(tb.lastRefillTime).Seconds()

	// Calculate tokens to add
	tokensToAdd := elapsedTime * tb.fillRate

	// Update token count and last refill time
	if tokensToAdd > 0 {
		tb.tokens = tb.tokens + tokensToAdd
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity
		}
		tb.lastRefillTime = now
	}
}

// Allow checks if a request is allowed based on rate limiting
func (tb *TokenBucket) Allow() bool {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()

	// Refill tokens based on elapsed time
	tb.refill()

	// Check if we have enough tokens
	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}

	return false
}

// IsAllowed checks if a request from a client is allowed
func (rl *RateLimiter) IsAllowed(clientID string) bool {
	limiter := rl.getLimiter(clientID)
	return limiter.Allow()
}

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("Error loading .env file\n" + err.Error())
	}

	// Initialize rate limiter with default values
	// These can be overridden via environment variables
	bucketCapacity := 10.0
	refillRate := 2.0

	// Check for environment variable overrides
	if val, exists := os.LookupEnv("RATE_LIMIT_CAPACITY"); exists {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			bucketCapacity = parsed
		}
	}

	if val, exists := os.LookupEnv("RATE_LIMIT_REFILL_RATE"); exists {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			refillRate = parsed
		}
	}

	// Create rate limiter instance
	rateLimiter = NewRateLimiter(bucketCapacity, refillRate)
	log.Printf("Rate limiter initialized with capacity: %.1f, refill rate: %.1f per second", bucketCapacity, refillRate)

	for {
		var err error
		dbconn, err = pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
		if err != nil {
			log.Printf("Unable to create connection pool: %v\n", err)
			time.Sleep(time.Second)
			continue
		}
		log.Printf("Connections to database created successfully\n")
		break
	}
	defer dbconn.Close()

	{
		type UserInfo struct {
			Client_id     string   `json:"client_id"`
			Client_secret string   `json:"client_secret"`
			Scope         []string `json:"scope"`
		}
		file, err := os.ReadFile("users.json")
		if err != nil {
			log.Fatalln("Error while reading users: ", err.Error())
		}
		var users []UserInfo
		if err := json.Unmarshal(file, &users); err != nil {
			log.Fatalln("Error while reading users: ", err.Error())
		}
		for _, user := range users {
			dbconn.Exec(context.Background(), "insert into public.user(client_id, client_secret, scope) values ($1, $2, $3)", user.Client_id, user.Client_secret, user.Scope)
		}

	}

	GetAllUsers()

	if os.Getenv("RELEASE") == "true" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// Rate limiting middleware
	rateLimitMiddleware := func(c *gin.Context) {
		// For token endpoint, get client ID from form
		clientID := ""
		if c.Request.URL.Path == "/token/" {
			clientID = c.PostForm("client_id")
		} else if c.Request.URL.Path == "/check/" {
			// For check endpoint, extract client ID from token
			authHeader := c.GetHeader("Authorization")
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				token := parts[1]
				// Extract client ID from token using our existing function
				if tokenItem, exists := tokens.Load(token); exists {
					tokenInfo := tokenItem.(TokenInfo)
					clientID = tokenInfo.ClientID
				}
			}
		}

		// If we couldn't identify the client, use IP address as fallback
		if clientID == "" {
			clientID = c.ClientIP()
		}

		// Check if request is allowed
		if !rateLimiter.IsAllowed(clientID) {
			// Add standard rate limiting headers
			c.Header("Retry-After", "1")
			c.Header("X-RateLimit-Limit", strconv.FormatFloat(rateLimiter.capacity, 'f', 0, 64))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Unix()+1, 10))

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       ErrRateLimited.Error(),
				"retry_after": "1", // Suggest retry after 1 second
			})
			c.Abort()
			return
		}

		c.Next()
	}

	// Apply rate limiting middleware to all endpoints
	r.Use(rateLimitMiddleware)

	// Get instance port from environment
	instancePort := os.Getenv("APP_PORT")
	if instancePort == "" {
		log.Fatal("APP_PORT environment variable is required")
	}
	// // middleware that logs the current instance
	// r.Use(func(c *gin.Context) {
	// 	log.Printf("Instance on port %s handling request: %s %s", instancePort, c.Request.Method, c.Request.URL.Path)
	// 	c.Next()
	// })

	r.POST("token/", func(ctx *gin.Context) {
		type Form struct {
			ClientId     string `form:"client_id" binding:"required"`
			Scope        string `form:"scope" binding:"required"`
			ClientSecret string `form:"client_secret" binding:"required"`
			GrantType    string `form:"grant_type" binding:"required"`
		}
		var f Form
		if err := ctx.ShouldBind(&f); err != nil {
			log.Println(err.Error())
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Missing some of following form fields: client_id, scope, client_secret, grant_type"})
			return
		}
		if f.GrantType != "client_credentials" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Incorrect grant type"})
			return
		}
		item, user_ok := users.Load(f.ClientId)
		user := item.(User)
		if !user_ok || f.ClientSecret != user.ClientSecret {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Incorrect client credentials"})
			return
		}
		is_pos_scope := false
		for i := range user.Scopes {
			if user.Scopes[i] == f.Scope {
				is_pos_scope = true
				break
			}
		}
		if !is_pos_scope {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Wrong scope"})
			return
		}
		token := AddToken(ctx, f.ClientId, f.Scope)
		if token == "" {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{
			"access_token":   token,
			"expires_in":     7200,
			"refresh_token":  "",
			"scope":          f.Scope,
			"security_level": "normal",
			"token_type":     "Bearer",
		})
	})
	r.GET("check/", func(ctx *gin.Context) {
		header := ctx.GetHeader("Authorization")
		ar := strings.Split(header, " ")
		if len(ar) != 2 || ar[0] != "Bearer" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Incorrect 'Authorization' header"})
			return
		}
		client_id, score, err := CheckToken(ctx, ar[1])
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{
			"client_id": client_id,
			"scope":     score,
		})
	})
	log.Println("Server started")

	//obtaining port from env
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8000"
	}
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
