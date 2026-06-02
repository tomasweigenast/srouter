package handler

import (
	"database/sql"
	"html/template"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do/v2"

	"github.com/tomasweigenast/srouter/internal/logging"
	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/internal/system"
	"github.com/tomasweigenast/srouter/web"
)

var wolLogger = logging.GetLogger("wol")

type WoLHandler struct {
	db   *sql.DB
	wol  system.WoL
	tmpl *template.Template
}

func NewWoLHandler(i do.Injector) (*WoLHandler, error) {
	return &WoLHandler{
		db:   do.MustInvoke[*sql.DB](i),
		wol:  do.MustInvoke[system.WoL](i),
		tmpl: web.MustParsePage("wol"),
	}, nil
}

func (h *WoLHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.show)
	r.Post("/devices", h.addDevice)
	r.Delete("/devices/{id}", h.deleteDevice)
	r.Post("/send/{id}", h.sendPacket)
	return r
}

type wolPage struct {
	ActivePage string
	Username   string
	Devices    []system.WoLDevice
}

func (h *WoLHandler) show(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	devices, _ := system.ListWoLDevices(h.db)
	web.Render(w, h.tmpl, wolPage{
		ActivePage: "wol",
		Username:   sess.Username,
		Devices:    devices,
	})
}

func (h *WoLHandler) addDevice(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	d := system.WoLDevice{
		Name: r.FormValue("name"),
		MAC:  r.FormValue("mac"),
		IP:   r.FormValue("ip"),
	}
	if d.Name == "" || d.MAC == "" {
		http.Error(w, "name and mac required", http.StatusBadRequest)
		return
	}
	if err := validateMAC(d.MAC); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id, err := system.AddWoLDevice(h.db, d)
	if err != nil {
		wolLogger.Error("add wol device", "err", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	d.ID = id
	html, _ := web.RenderPartial(h.tmpl, "wol_device_row", d)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (h *WoLHandler) deleteDevice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := system.DeleteWoLDevice(h.db, id); err != nil {
		wolLogger.Error("delete wol device", "id", id, "err", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *WoLHandler) sendPacket(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	devices, _ := system.ListWoLDevices(h.db)
	var mac string
	for _, d := range devices {
		if d.ID == id {
			mac = d.MAC
			break
		}
	}
	if mac == "" {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}
	if err := h.wol.SendMagicPacket(mac); err != nil {
		wolLogger.Error("send magic packet", "mac", mac, "err", err)
		writeJSON(w, false, "Failed: "+err.Error())
		return
	}
	wolLogger.Info("magic packet sent", "mac", mac)
	writeJSON(w, true, "Magic packet sent!")
}
