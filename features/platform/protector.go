package platform

// Protector excludes sockets from the active VPN tunnel (Android VpnService.protect).
type Protector interface {
	Protect(socket int32) bool
	GetResolvers() string
}
