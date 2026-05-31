package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	chi "github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/tomasweigenast/srouter/internal/config"
	"github.com/tomasweigenast/srouter/internal/db"
	"github.com/tomasweigenast/srouter/internal/env"
	"github.com/tomasweigenast/srouter/internal/handler"
	"github.com/tomasweigenast/srouter/internal/logging"
	appmw "github.com/tomasweigenast/srouter/internal/middleware"
	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/internal/system"
	"github.com/tomasweigenast/srouter/web"
)

func main() {
	logger := logging.GetLogger("main")

	cfg, err := config.Load(env.ConfigPath())
	if err != nil {
		logger.Error("loading config", "err", err)
		os.Exit(1)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		logger.Error("opening database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	// Background workers
	go session.CleanupLoop(database, time.Hour)

	bwBroadcast := system.NewBroadcaster(time.Second)
	defer bwBroadcast.Stop()

	logBroadcast := system.NewLogBroadcaster("/var/log/messages")
	defer logBroadcast.Stop()

	// Handlers
	authHandler := handler.NewAuthHandler(database)
	dashboardHandler := handler.NewDashboardHandler()
	dhcpHandler := handler.NewDHCPHandler()
	dnsHandler := handler.NewDNSHandler()
	networkHandler := handler.NewNetworkHandler()
	firewallHandler := handler.NewFirewallHandler()
	portForwardHandler := handler.NewPortForwardHandler()
	logsHandler := handler.NewLogsHandler(logBroadcast)
	bandwidthHandler := handler.NewBandwidthHandler(bwBroadcast)
	wolHandler := handler.NewWoLHandler(database)

	// Router
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)

	// Public routes
	authHandler.Register(r)

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(appmw.RequireAuth(database))
		dashboardHandler.Register(r)
		r.Mount("/dhcp", dhcpHandler.Routes())
		r.Mount("/dns", dnsHandler.Routes())
		r.Mount("/network", networkHandler.Routes())
		r.Mount("/firewall", firewallHandler.Routes())
		r.Mount("/portforward", portForwardHandler.Routes())
		r.Mount("/logs", logsHandler.Routes())
		r.Mount("/bandwidth", bandwidthHandler.Routes())
		r.Mount("/wol", wolHandler.Routes())
	})

	// Static files
	r.Handle("/static/*", http.StripPrefix("/static/",
		http.FileServer(http.FS(web.StaticFS))))

	// Root redirect
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	})

	srv := &http.Server{
		Addr:         cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("server starting", "addr", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-stop
	logger.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "err", err)
	}
}
