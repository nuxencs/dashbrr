// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package routes

import (
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/dashbrr/internal/api/handlers"
	"github.com/autobrr/dashbrr/internal/api/middleware"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/types"
)

// SetupRoutes configures all the routes for the application
func SetupRoutes(r *gin.Engine, db *database.DB, health *services.HealthService) cache.Store {
	// Use custom logger instead of default Gin logger
	r.Use(middleware.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.SetupCORS())
	r.Use(middleware.Secure(nil)) // Add secure middleware with default config

	// Initialize cache with database directory for session storage
	cacheConfig := cache.Config{
		DataDir: filepath.Dir(os.Getenv("DASHBRR__DB_PATH")), // Use same directory as database
	}

	// Configure Redis if enabled
	if os.Getenv("REDIS_HOST") != "" {
		host := os.Getenv("REDIS_HOST")
		port := os.Getenv("REDIS_PORT")
		if port == "" {
			port = "6379"
		}
		cacheConfig.RedisAddr = host + ":" + port
	}

	store, err := cache.InitCache(cacheConfig)
	if err != nil {
		// This should never happen as InitCache always returns a valid store
		log.Debug().Err(err).Msg("Using memory cache")
		store = cache.NewMemoryStore(cacheConfig.DataDir)
	}

	// Determine cache type based on environment and Redis configuration
	cacheType := "memory"
	if os.Getenv("CACHE_TYPE") == "redis" && os.Getenv("REDIS_HOST") != "" {
		cacheType = "redis"
	}
	log.Debug().Str("type", cacheType).Msg("Cache initialized")

	// Create rate limiters with different configurations
	apiRateLimiter := middleware.NewRateLimiter(store, time.Minute, 60, "api:")       // 60 requests per minute for API
	healthRateLimiter := middleware.NewRateLimiter(store, time.Minute, 30, "health:") // 30 health checks per minute
	authRateLimiter := middleware.NewRateLimiter(store, time.Minute, 30, "auth:")     // 30 auth requests per minute

	// Special rate limiter for Tailscale services
	tailscaleRateLimiter := middleware.NewRateLimiter(store, 2*time.Minute, 20, "tailscale:") // 20 requests per 2 minutes

	// Create cache middleware (now handles TTLs internally)
	cacheMiddleware := middleware.NewCacheMiddleware(store)

	// Initialize handlers with cache
	settingsHandler := handlers.NewSettingsHandler(db, health)
	healthHandler := handlers.NewHealthHandler(db, health)
	eventsHandler := handlers.NewEventsHandler(db, health)
	autobrrHandler := handlers.NewAutobrrHandler(db, store)
	omegabrrHandler := handlers.NewOmegabrrHandler(db, store)
	maintainerrHandler := handlers.NewMaintainerrHandler(db, store)
	plexHandler := handlers.NewPlexHandler(db, store)
	tailscaleHandler := handlers.NewTailscaleHandler(db, store)
	overseerrHandler := handlers.NewOverseerrHandler(db, store)
	sonarrHandler := handlers.NewSonarrHandler(db, store)
	radarrHandler := handlers.NewRadarrHandler(db, store)
	prowlarrHandler := handlers.NewProwlarrHandler(db, store)

	// Initialize auth handlers and middleware
	var oidcAuthHandler *handlers.AuthHandler
	builtinAuthHandler := handlers.NewBuiltinAuthHandler(db, store)
	authMiddleware := middleware.NewAuthMiddleware(store)

	// Initialize OIDC if configuration is provided
	if hasOIDCConfig() {
		authConfig := &types.AuthConfig{
			Issuer:       getEnvOrDefault("OIDC_ISSUER", ""),
			ClientID:     getEnvOrDefault("OIDC_CLIENT_ID", ""),
			ClientSecret: getEnvOrDefault("OIDC_CLIENT_SECRET", ""),
			RedirectURL:  getEnvOrDefault("OIDC_REDIRECT_URL", "http://localhost:3000/api/auth/callback"),
		}
		oidcAuthHandler = handlers.NewAuthHandler(authConfig, store)
	}

	// Start the health monitor
	eventsHandler.StartHealthMonitor()

	// Public routes (no auth required)
	public := r.Group("")
	{
		// Health check endpoint
		public.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		// Auth configuration endpoint
		public.GET("/api/auth/config", handlers.GetAuthConfig)

		// OIDC auth endpoints (only if OIDC is configured)
		if oidcAuthHandler != nil {
			public.GET("/api/auth/callback", oidcAuthHandler.Callback)
			oidcAuth := public.Group("/api/auth/oidc")
			oidcAuth.Use(authRateLimiter.RateLimit())
			{
				oidcAuth.GET("/login", oidcAuthHandler.Login)
				oidcAuth.POST("/logout", oidcAuthHandler.Logout)
			}
		}

		// Built-in auth endpoints
		builtinAuth := public.Group("/api/auth")
		builtinAuth.Use(authRateLimiter.RateLimit())
		{
			builtinAuth.GET("/registration-status", builtinAuthHandler.CheckRegistrationStatus)
			builtinAuth.POST("/register", builtinAuthHandler.Register)
			builtinAuth.POST("/login", builtinAuthHandler.Login)
			builtinAuth.POST("/logout", builtinAuthHandler.Logout)
			builtinAuth.GET("/verify", builtinAuthHandler.Verify)
		}
	}

	// Protected auth routes
	protectedAuth := r.Group("/api/auth")
	protectedAuth.Use(authMiddleware.RequireAuth())
	protectedAuth.Use(authRateLimiter.RateLimit())
	{
		if oidcAuthHandler != nil {
			oidc := protectedAuth.Group("/oidc")
			{
				oidc.POST("/refresh", oidcAuthHandler.RefreshToken)
				oidc.GET("/verify", oidcAuthHandler.VerifyToken)
				oidc.GET("/userinfo", oidcAuthHandler.UserInfo)
			}
		}
		protectedAuth.GET("/userinfo", builtinAuthHandler.GetUserInfo)
	}

	// API routes group with auth middleware
	api := r.Group("/api")
	api.Use(authMiddleware.RequireAuth())
	{
		// Settings endpoints - no caching to ensure fresh data
		settings := api.Group("/settings")
		{
			settings.GET("", settingsHandler.GetSettings)
			settings.POST("/:instance", settingsHandler.SaveSettings)
			settings.DELETE("/:instance", settingsHandler.DeleteSettings)
		}

		// Health check endpoints (no cache for SSE)
		health := api.Group("/health")
		health.Use(healthRateLimiter.RateLimit())
		{
			health.GET("/:service", healthHandler.CheckHealth)
			health.GET("/events", eventsHandler.StreamHealth)
		}

		// Service endpoints with specific rate limits and caches
		services := api.Group("")
		{
			// Regular services with standard rate limit
			regularServices := services.Group("")
			regularServices.Use(apiRateLimiter.RateLimit())
			regularServices.Use(cacheMiddleware.Cache())
			{
				regularServices.GET("/autobrr/stats", autobrrHandler.GetAutobrrReleaseStats)
				regularServices.GET("/autobrr/irc", autobrrHandler.GetAutobrrIRCStatus)
				regularServices.GET("/plex/sessions", plexHandler.GetPlexSessions)
				regularServices.GET("/maintainerr/collections", maintainerrHandler.GetMaintainerrCollections)

				// Overseerr endpoints
				overseerr := regularServices.Group("/overseerr")
				{
					overseerr.GET("/requests", overseerrHandler.GetRequests)
				}

				// Sonarr endpoints
				sonarr := regularServices.Group("/sonarr")
				{
					sonarr.GET("/queue", sonarrHandler.GetQueue)
					sonarr.GET("/stats", sonarrHandler.GetStats)
					sonarr.DELETE("/queue/:id", sonarrHandler.DeleteQueueItem)
				}

				// Radarr endpoints
				radarr := regularServices.Group("/radarr")
				{
					radarr.GET("/queue", radarrHandler.GetQueue)
					radarr.DELETE("/queue/:id", radarrHandler.DeleteQueueItem)
				}

				// Prowlarr endpoints
				prowlarr := regularServices.Group("/prowlarr")
				{
					prowlarr.GET("/stats", prowlarrHandler.GetStats)
					prowlarr.GET("/indexers", prowlarrHandler.GetIndexers)
				}

				// Omegabrr endpoints
				omegabrr := regularServices.Group("/omegabrr")
				{
					omegabrr.GET("/status", omegabrrHandler.GetOmegabrrStatus)
					webhook := omegabrr.Group("/webhook")
					{
						webhook.POST("/arrs", omegabrrHandler.TriggerWebhookArrs)
						webhook.POST("/lists", omegabrrHandler.TriggerWebhookLists)
						webhook.POST("/all", omegabrrHandler.TriggerWebhookAll)
					}
				}
			}

			// Tailscale services with special rate limit
			tailscaleServices := services.Group("")
			tailscaleServices.Use(tailscaleRateLimiter.RateLimit())
			tailscaleServices.Use(cacheMiddleware.Cache())
			{
				tailscaleServices.GET("/tailscale/devices", tailscaleHandler.GetTailscaleDevices)
			}

			// Service action endpoints that require instanceId
			serviceActions := services.Group("/services/:instanceId")
			serviceActions.Use(apiRateLimiter.RateLimit())
			{
				// Overseerr action endpoints
				overseerrActions := serviceActions.Group("/overseerr")
				{
					overseerrActions.POST("/request/:requestId/:status", overseerrHandler.UpdateRequestStatus)
				}
			}
		}
	}

	return store
}

// hasOIDCConfig checks if all required OIDC configuration is provided
func hasOIDCConfig() bool {
	return os.Getenv("OIDC_ISSUER") != "" &&
		os.Getenv("OIDC_CLIENT_ID") != "" &&
		os.Getenv("OIDC_CLIENT_SECRET") != ""
}

// getEnvOrDefault returns the value of an environment variable or a default value if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
