package trader

// maskWalletAddress masks wallet address, showing only first 6 and last 4 characters
func maskWalletAddress(addr string) string {
	if addr == "" {
		return ""
	}
	length := len(addr)
	if length <= 10 {
		return "****" // Address too short, hide all
	}
	return addr[:6] + "..." + addr[length-4:]
}

