package boundary

// CLIRequest are common things that we need when making a CLI request.
type CLIRequest struct {
	BinName    string
	BinPath    string
	ConfigPath string
	License    string
}
