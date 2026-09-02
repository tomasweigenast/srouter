package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"strings"
	"time"

	"github.com/tomasweigenast/srouter/internal/system"
)

//go:embed templates static
var assetsFS embed.FS

// TemplatesFS is the embedded templates filesystem.
var TemplatesFS, _ = fs.Sub(assetsFS, "templates")

// StaticFS is the embedded static files filesystem.
var StaticFS, _ = fs.Sub(assetsFS, "static")

// NavItem is passed to the navlink template.
type NavItem struct {
	Href   string
	ID     string
	Label  string
	Active string
}

// FuncMap holds shared template functions.
var FuncMap = template.FuncMap{
	"navItem": func(href, id, label, active string) NavItem {
		return NavItem{Href: href, ID: id, Label: label, Active: active}
	},
	// mb converts KB to MB as an integer string.
	"mb": func(kb uint64) string { return fmt.Sprintf("%d", kb/1024) },
	// gb converts bytes to GB with one decimal.
	"gb": func(b uint64) string { return fmt.Sprintf("%.1f", float64(b)/1e9) },
	// macID converts a MAC address (aa:bb:cc) to a CSS-safe ID (aa-bb-cc).
	"macID": func(mac string) string { return strings.ReplaceAll(mac, ":", "-") },
	// bandwidthPresets returns the allowed download limit values in Mbps.
	"bandwidthPresets": func() []int { return system.AllowedBandwidthMbps },
	// uptimeFmt formats seconds into "Xd Yh Zm" string.
	"uptimeFmt": func(secs int64) string {
		d := secs / 86400
		h := (secs % 86400) / 3600
		m := (secs % 3600) / 60
		if d > 0 {
			return fmt.Sprintf("%dd %dh %dm", d, h, m)
		}
		if h > 0 {
			return fmt.Sprintf("%dh %dm", h, m)
		}
		return fmt.Sprintf("%dm", m)
	},
	// uptimeSince returns a human-readable "X ago" string for a past time.
	"uptimeSince": func(t time.Time) string {
		d := time.Since(t)
		switch {
		case d < time.Minute:
			return "just now"
		case d < time.Hour:
			return fmt.Sprintf("%dm ago", int(d.Minutes()))
		case d < 24*time.Hour:
			return fmt.Sprintf("%dh ago", int(d.Hours()))
		default:
			return fmt.Sprintf("%dd ago", int(d.Hours()/24))
		}
	},
}
