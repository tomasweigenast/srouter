package system

import (
	"database/sql"

	"github.com/samber/do/v2"
)

// ProvideSystem registers all system interfaces in the DI injector,
// pointing at RealSystem (production) or MockSystem (dev mode on macOS).
// Called once in cmd/main.go before registering handlers.
func ProvideSystem(i do.Injector, db *sql.DB, devMode bool) {
	var impl interface {
		Metrics
		DHCP
		DNS
		Network
		Firewall
		WoL
	}
	if devMode {
		impl = MockSystem{}
	} else {
		impl = RealSystem{}
	}

	do.ProvideValue[Metrics](i, impl)
	do.ProvideValue[DHCP](i, impl)
	do.ProvideValue[DNS](i, impl)
	do.ProvideValue[Network](i, impl)
	do.ProvideValue[Firewall](i, impl)
	do.ProvideValue[WoL](i, impl)

	if devMode {
		do.ProvideValue[DoH](i, MockDoHSystem{})
	} else {
		do.ProvideValue[DoH](i, RealDoHSystem{DB: db})
	}
}
