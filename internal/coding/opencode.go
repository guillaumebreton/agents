package coding

// OpenCode is the coding agent implementation for opencode.
type OpenCode struct{}

func init() {
	Register(&OpenCode{})
}

func (o *OpenCode) Name() string {
	return "opencode"
}

func (o *OpenCode) Command() string {
	return "opencode"
}
