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
	"github.com/samber/do/v2"

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

	// ── DI container ────────────────────────────────────────────────────
	i := do.New()

	// Primitives
	do.ProvideValue(i, cfg)
	do.ProvideValue(i, database)

	// System interfaces — real or mock based on SROUTER_DEV_MODE
	system.ProvideSystem(i, database, cfg.DevMode)

	// Update checker
	do.ProvideValue(i, system.NewUpdateChecker(cfg.DevMode))

	// Reboot reset channel — buffered so the handler never blocks.
	rebootCh := make(chan struct{}, 1)
	do.ProvideValue[chan<- struct{}](i, rebootCh)

	// SSE streams
	do.Provide(i, func(i do.Injector) (system.LogStream, error) {
		cfg := do.MustInvoke[config.Config](i)
		if cfg.DevMode {
			return system.NewMockLogStream(), nil
		}
		return system.NewLogBroadcaster("/var/log/messages"), nil
	})
	do.Provide(i, func(i do.Injector) (system.BandwidthStream, error) {
		cfg := do.MustInvoke[config.Config](i)
		interval := time.Duration(cfg.UpdateIntervalMs) * time.Millisecond
		if cfg.DevMode {
			return system.MockBandwidthStream{}, nil
		}
		return system.NewBroadcaster(interval), nil
	})

	// Handlers
	do.Provide(i, handler.NewAuthHandler)
	do.Provide(i, handler.NewDashboardHandler)
	do.Provide(i, handler.NewDHCPHandler)
	do.Provide(i, handler.NewDNSHandler)
	do.Provide(i, handler.NewNetworkHandler)
	do.Provide(i, handler.NewFirewallHandler)
	do.Provide(i, handler.NewPortForwardHandler)
	do.Provide(i, handler.NewLogsHandler)
	do.Provide(i, handler.NewBandwidthHandler)
	do.Provide(i, handler.NewWoLHandler)
	do.Provide(i, handler.NewSpeedtestHandler)
	do.Provide(i, handler.NewSystemHandler)
	do.Provide(i, handler.NewDoHHandler)
	do.Provide(i, func(i do.Injector) (*system.DNSStatsCollector, error) {
		logs := do.MustInvoke[system.LogStream](i)
		if do.MustInvoke[config.Config](i).DevMode {
			return system.NewMockDNSStatsCollector(), nil
		}
		return system.NewDNSStatsCollector(logs), nil
	})

	// ── Background workers ───────────────────────────────────────────────
	go session.CleanupLoop(database, time.Hour)
	go system.RebootWatchLoop(database, rebootCh)
	go system.UpdateCheckLoop(do.MustInvoke[*system.UpdateChecker](i), 6*time.Hour)

	// ── HTTP router ──────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "same-origin")
			next.ServeHTTP(w, r)
		})
	})
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10 MB
			next.ServeHTTP(w, r)
		})
	})

	authHandler      := do.MustInvoke[*handler.AuthHandler](i)
	dashboardHandler := do.MustInvoke[*handler.DashboardHandler](i)
	dhcpHandler      := do.MustInvoke[*handler.DHCPHandler](i)
	dnsHandler       := do.MustInvoke[*handler.DNSHandler](i)
	networkHandler   := do.MustInvoke[*handler.NetworkHandler](i)
	firewallHandler  := do.MustInvoke[*handler.FirewallHandler](i)
	pfHandler        := do.MustInvoke[*handler.PortForwardHandler](i)
	logsHandler      := do.MustInvoke[*handler.LogsHandler](i)
	bwHandler        := do.MustInvoke[*handler.BandwidthHandler](i)
	wolHandler          := do.MustInvoke[*handler.WoLHandler](i)
	speedtestHandler    := do.MustInvoke[*handler.SpeedtestHandler](i)
	systemHandler       := do.MustInvoke[*handler.SystemHandler](i)
	doHHandler          := do.MustInvoke[*handler.DoHHandler](i)

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
		r.Mount("/portforward", pfHandler.Routes())
		r.Mount("/logs", logsHandler.Routes())
		r.Mount("/bandwidth", bwHandler.Routes())
		r.Mount("/wol", wolHandler.Routes())
		r.Mount("/speedtest", speedtestHandler.Routes())
		r.Mount("/system", systemHandler.Routes())
		r.Mount("/doh", doHHandler.Routes())
	})

	// Static files
	r.Handle("/static/*", http.StripPrefix("/static/",
		http.FileServer(http.FS(web.StaticFS))))

	// Unauthenticated liveness probe used by the update UI to detect restarts.
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Root redirect
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	})

	// ── HTTP server ──────────────────────────────────────────────────────
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
		logger.Info("server starting", "addr", cfg.Port, "dev", cfg.DevMode)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-stop
	logger.Info("shutting down")

	// Close SSE streams first so their goroutines unblock and HTTP server
	// connections drain immediately — otherwise Shutdown waits the full timeout.
	_ = i.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "err", err)
	}
}
