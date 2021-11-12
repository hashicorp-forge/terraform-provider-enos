package vault

// CLIRequest are common things that we need when making a CLI request
type CLIRequest struct {
	VaultAddr string
	Token     string
	BinPath   string
}
